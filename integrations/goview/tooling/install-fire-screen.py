"""Publish the authored screen into the deployment database.

The API is the normal path, but it needs an interactive login. Writing the two
rows the API would write (the screen draft plus its resource-reference rows, so
delete protection and authorization still see the bound devices) is equivalent
and keeps this step scriptable.

    python tooling/install-fire-screen.py            # dry run
    python tooling/install-fire-screen.py --write    # insert
"""
import argparse
import datetime
import json
import pathlib
import shutil
import sqlite3
import sys

DATA = pathlib.Path('C:/Users/Administrator/Documents/noyo/data')
DB = DATA / 'db' / 'noyo.db'
HERE = pathlib.Path(__file__).resolve().parent
DOCUMENT = HERE.parent / 'tmp-verify' / 'fixtures' / 'fire-screen.json'
BACKUP = HERE.parent / 'tmp-verify' / 'backup-ioc-screens.json'

NAME = '消防设施物联网监控中心'
TENANT_ID, PROJECT_ID, USER_ID = 1, 1, 1


def references(document):
    rows = set()
    for binding in document['bindings'].values():
        devices = list(binding.get('deviceCodes') or [])
        if binding.get('deviceCode'):
            devices.append(binding['deviceCode'])
        keys = list(binding.get('keys') or [])
        if binding.get('key'):
            keys.append(binding['key'])
        for device in devices:
            rows.add((device, ''))
        for key in keys:
            rows.add((devices[0], key))
    return sorted(rows)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--write', action='store_true')
    args = parser.parse_args()

    document = json.loads(DOCUMENT.read_text(encoding='utf-8'))
    payload = json.dumps(document, ensure_ascii=False, separators=(',', ':'))
    refs = references(document)
    print(f'name={NAME} components={len(document["content"]["componentList"])} '
          f'bindings={len(document["bindings"])} refs={len(refs)} bytes={len(payload.encode("utf-8"))}')

    if not args.write:
        for device, key in refs:
            print('  ref', device, key or '-')
        return

    con = sqlite3.connect(DB)
    con.row_factory = sqlite3.Row
    try:
        existing = [dict(row) for row in con.execute(
            'select id, name, draft_version, draft_json from ioc_screens')]
        BACKUP.write_text(json.dumps(existing, ensure_ascii=False, indent=1), encoding='utf-8')
        shutil.copy2(DB, DB.with_suffix('.db.bak-before-fire-screen'))
        print(f'backup -> {BACKUP} and {DB.name}.bak-before-fire-screen')

        duplicate = con.execute('select id from ioc_screens where name = ? and deleted_at is null',
                                (NAME,)).fetchone()
        now = datetime.datetime.now().astimezone().isoformat()
        if duplicate:
            screen_id = duplicate['id']
            con.execute('update ioc_screens set draft_json = ?, draft_version = draft_version + 1, '
                        'updated_by = ?, updated_at = ? where id = ?',
                        (payload, USER_ID, now, screen_id))
            print(f'updated screen id={screen_id}')
        else:
            cursor = con.execute(
                'insert into ioc_screens (tenant_id, project_id, name, metadata_version, draft_json, '
                'draft_version, published_release_id, created_by, updated_by, created_at, updated_at, '
                'last_restore_request_id, last_restore_release_id, last_restore_base_version, '
                'last_restore_version, copy_source_id, copy_request_id, copy_request_hash) '
                'values (?, ?, ?, 1, ?, 1, null, ?, ?, ?, ?, "", 0, 0, 0, 0, null, "")',
                (TENANT_ID, PROJECT_ID, NAME, payload, USER_ID, USER_ID, now, now))
            screen_id = cursor.lastrowid
            print(f'inserted screen id={screen_id}')

        con.execute('delete from ioc_screen_resource_refs where owner_kind = ? and owner_id = ?',
                    ('draft', screen_id))
        con.executemany(
            'insert into ioc_screen_resource_refs (tenant_id, project_id, owner_kind, owner_id, '
            'resource_type, resource_id, field_key) values (?, ?, ?, ?, ?, ?, ?)',
            [(TENANT_ID, PROJECT_ID, 'draft', screen_id, 'device', device, key) for device, key in refs])
        con.commit()
        print(f'refs written: {len(refs)}')
    except Exception as error:
        con.rollback()
        print('FAILED:', error)
        sys.exit(1)
    finally:
        con.close()


if __name__ == '__main__':
    main()
