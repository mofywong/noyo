"""Capture the live runtime payloads of an installed screen into the render fixture.

The harness can be fed either hand-derived samples (make-screen-fixture.py reads
the databases directly) or -- as here -- exactly what the running server returns
for the published release, so the preview cannot drift from the wall screen.

    python tooling/capture-live-fixture.py --base http://[::1]:8989 --screen 2 --token "$NOYO_TOKEN"
"""
import argparse
import json
import pathlib
import urllib.error
import urllib.request

HERE = pathlib.Path(__file__).resolve().parent
FIXTURES = HERE.parent / 'tmp-verify' / 'fixtures'
DOCUMENT = FIXTURES / 'fire-screen.json'


def post(base, screen, path, token, payload):
    request = urllib.request.Request(
        f'{base}/api/screens/{screen}/runtime/{path}',
        data=json.dumps(payload).encode('utf-8'), method='POST',
        headers={'Content-Type': 'application/json', 'Authorization': f'Bearer {token}',
                 'X-Current-Tenant-ID': '1', 'X-Current-Project-ID': '1'})
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    with opener.open(request, timeout=20) as response:
        body = json.loads(response.read().decode('utf-8'))
    if body.get('code') != 0:
        raise SystemExit(f'{path}: {body.get("code")} {body.get("message")}')
    return body['data']


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--base', default='http://[::1]:8989')
    parser.add_argument('--screen', type=int, default=2)
    parser.add_argument('--token', required=True)
    args = parser.parse_args()

    document = json.loads(DOCUMENT.read_text(encoding='utf-8'))
    # The snapshot endpoint already answers with exactly the document's bindings,
    # so it doubles as the `runtime.batch` payload the host pushes.
    samples = post(args.base, args.screen, 'snapshot', args.token, {})['bindings']
    history = {}
    for binding_id, binding in document['bindings'].items():
        endpoint = {'history': 'history', 'alarmList': 'alarms'}.get(binding['kind'])
        if not endpoint:
            continue
        data = post(args.base, args.screen, endpoint, args.token, {'bindingId': binding_id})
        history[binding_id] = {'value': data['value'], 'quality': data['quality']}

    payload = {'revision': 'published', 'runtimeEpoch': 'live-capture', 'serverTime': 0,
               'bindings': samples}
    (FIXTURES / 'screen-samples.json').write_text(json.dumps(payload, ensure_ascii=False), encoding='utf-8')
    # The harness resolves `fixtureHistory[bindingId]` at top level.
    (FIXTURES / 'screen-history.json').write_text(json.dumps(history, ensure_ascii=False), encoding='utf-8')
    print('samples', {key: (value['value'] if not isinstance(value['value'], list) else f'{len(value["value"])} rows')
                      for key, value in samples.items()})
    print('history', {key: len(value['value']) for key, value in history.items()})


if __name__ == '__main__':
    main()
