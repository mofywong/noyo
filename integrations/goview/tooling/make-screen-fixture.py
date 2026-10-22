"""Build a render fixture from the real Noyo database.

Reads the authored screen document, then fills every binding with data taken
from the deployment database (device_locations, alarm_instances) and the
monthly history shards, so the harness renders the screen with real values
instead of pictures of numbers.

    python tooling/make-screen-fixture.py
"""
import json
import pathlib
import sqlite3
import time

DATA = pathlib.Path('C:/Users/Administrator/Documents/noyo/data')
DB = DATA / 'db' / 'noyo.db'
HISTORY = DATA / 'db' / 'history'

HERE = pathlib.Path(__file__).resolve().parent
FIXTURES = HERE.parent / 'tmp-verify' / 'fixtures'
DOCUMENT = FIXTURES / 'fire-screen.json'


def connect(path, readonly=True):
    mode = '?mode=ro' if readonly else ''
    con = sqlite3.connect(f'file:{path}{mode}', uri=True)
    con.row_factory = sqlite3.Row
    return con


def history_shards():
    return sorted(HISTORY.glob('history_*.db'), reverse=True)


def latest_sample(device_code, key):
    for shard in history_shards():
        if shard.stat().st_size < 20000:
            continue
        con = connect(shard)
        try:
            for row in con.execute(
                'select ts, payload from records where device_code = ? and type = 1 '
                'order by ts desc limit 400', (device_code,)):
                try:
                    payload = json.loads(row['payload'])
                except Exception:
                    continue
                if key in payload and payload[key] not in (None, ''):
                    # The runtime only exposes a cached live value; a sample the
                    # device stopped reporting minutes ago is no_data, not a
                    # number the screen should keep showing.
                    if time.time() * 1000 - int(row['ts']) > 5 * 60000:
                        return None, 0
                    return payload[key], int(row['ts'])
        finally:
            con.close()
    return None, 0


def history_series(device_code, keys, points=240, window_ms=7 * 86400000):
    for shard in history_shards():
        if shard.stat().st_size < 20000:
            continue
        con = connect(shard)
        try:
            rows = list(con.execute(
                'select ts, payload from records where device_code = ? and type = 1 '
                'order by ts desc limit 30000', (device_code,)))
        finally:
            con.close()
        if not rows:
            continue
        rows.reverse()
        collected = []
        for row in rows:
            try:
                payload = json.loads(row['payload'])
            except Exception:
                continue
            if not any(key in payload for key in keys):
                continue
            collected.append((int(row['ts']), payload))
        if not collected:
            continue
        # The server windows against "now", not against the last stored sample,
        # so the fixture must do the same or the preview shows data the live
        # screen will never receive.
        end = int(time.time() * 1000)
        sliced = [item for item in collected if item[0] >= end - window_ms]
        step = max(1, len(sliced) // points)
        out = []
        for timestamp, payload in sliced[::step][:points]:
            entry = {'ts': timestamp}
            for key in keys:
                value = payload.get(key)
                if isinstance(value, (int, float)):
                    entry[key] = round(float(value), 2)
            if len(entry) > 1:
                out.append(entry)
        if out:
            return out
    return []


def main():
    document = json.loads(DOCUMENT.read_text(encoding='utf-8'))
    con = connect(DB)

    locations = {}
    for row in con.execute('select device_code, latitude, longitude, source from device_locations'):
        locations[row['device_code']] = row

    alarms = []
    for row in con.execute(
            'select public_id, title, severity, condition_status, handling_status, '
            'last_occurred_at, evidence_snapshot from alarm_instances '
            'where handling_status <> ? order by last_occurred_at desc limit 60', ('closed',)):
        try:
            evidence = json.loads(row['evidence_snapshot'] or '{}')
        except Exception:
            evidence = {}
        alarms.append({
            'id': row['public_id'], 'deviceCode': evidence.get('device_code', '—'),
            'deviceName': evidence.get('device_name', ''),
            'title': row['title'], 'severity': row['severity'],
            'conditionStatus': row['condition_status'],
            'handlingStatus': row['handling_status'],
            'occurredAt': row['last_occurred_at'],
        })
    con.close()

    samples, histories = {}, {}
    for binding_id, binding in document['bindings'].items():
        kind = binding['kind']
        if kind in ('history',):
            histories[binding_id] = {'value': history_series(
                binding['deviceCode'], binding.get('keys') or [binding.get('key')],
                points=binding.get('maxPoints') or 240,
                window_ms=binding.get('windowMs') or 7 * 86400000), 'quality': 'good'}
            continue
        if kind == 'alarmList':
            # Mirror the runtime RPC: an alarmList binding only receives alarms
            # whose device is inside the binding's device scope.
            scope = set(binding.get('deviceCodes') or [])
            scoped = [item for item in alarms if not scope or item['deviceCode'] in scope]
            histories[binding_id] = {'value': scoped, 'quality': 'good'}
            continue
        if kind == 'location':
            points = []
            for code in binding['deviceCodes']:
                row = locations.get(code)
                if not row:
                    continue
                points.append({'deviceCode': code, 'latitude': row['latitude'],
                               'longitude': row['longitude'], 'coordinateSystem': 'wgs84',
                               'source': row['source'], 'quality': 'good'})
            samples[binding_id] = {'value': points, 'quality': 'good',
                                   'receivedAt': 0, 'sampleVersion': 1}
            continue
        value, stamp = latest_sample(binding['deviceCode'], binding.get('key'))
        samples[binding_id] = {'value': value, 'quality': 'good' if value is not None else 'no_data',
                               'receivedAt': stamp, 'sampleVersion': 1}

    revision = 'draft-1'
    payload = {
        'revision': revision,
        'runtimeEpoch': 'fixture-epoch',
        'serverTime': 0,
        'bindings': samples,
    }
    (FIXTURES / 'screen-samples.json').write_text(json.dumps(payload, ensure_ascii=False), encoding='utf-8')
    (FIXTURES / 'screen-history.json').write_text(json.dumps(histories, ensure_ascii=False), encoding='utf-8')

    print('samples:', {k: (v['value'] if not isinstance(v['value'], list) else f'{len(v["value"])} rows') for k, v in samples.items()})
    print('history:', {k: (len(v['value']) if isinstance(v['value'], list) else v['value']) for k, v in histories.items()})


if __name__ == '__main__':
    main()
