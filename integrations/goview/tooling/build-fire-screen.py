"""Compose the fire-protection control-room screen from reviewed widget defaults.

The defaults are dumped from the running catalogue (dump-widget-defaults.mjs) so
chartKey / conKey / package always match the backend's reviewed keys. Layout,
copy and bindings are authored here; nothing is hand-typed that the backend
validates.

    python tooling/build-fire-screen.py            # write the document
    python tooling/build-fire-screen.py --check    # validate with Go as well
"""
import argparse
import json
import pathlib
import subprocess
import sys

ROOT = pathlib.Path(__file__).resolve().parents[3]
HERE = pathlib.Path(__file__).resolve().parent
DEFAULTS = HERE.parent / 'tmp-verify' / 'fixtures' / 'widget-defaults.json'
OUT = HERE.parent / 'tmp-verify' / 'fixtures' / 'fire-screen.json'

ENGINE = {
    'name': 'goview',
    'buildId': 'goview-5ab05b4-noyo8',
    'upstreamSha': '5ab05b439703ffda6d050e5626969459b3e3c1cf',
}
ENGINE_BUILD = 'goview-5ab05b4-noyo8'

CANVAS = {'width': 1920, 'height': 1080, 'scaleMode': 'contain'}

# Dark control-room canvas; the editor stores this inside content.
EDIT_CANVAS = {
    'width': 1920, 'height': 1080, 'scaleMode': 'fit',
    'background': '#050b16', 'backgroundImage': '',
    'selectColor': '#00baff', 'chartThemeColor': 'dark',
}



def wrap(text, width):
    """Hard-wrap CJK copy: pre-wrap only breaks on spaces, and Chinese has none."""
    lines = []
    for paragraph in text.split('\n'):
        if not paragraph:
            lines.append('')
            continue
        while len(paragraph) > width:
            lines.append(paragraph[:width])
            paragraph = paragraph[width:]
        lines.append(paragraph)
    return '\n'.join(lines)

def load_defaults():
    data = json.loads(DEFAULTS.read_text(encoding='utf-8'))
    return {item['key']: item for item in data}


def component(defaults, key, cid, x, y, w, h, title=None, options=None, z=0):
    base = json.loads(json.dumps(defaults[key]))
    base['id'] = cid
    base['attr'] = {'x': x, 'y': y, 'w': w, 'h': h, 'offsetX': 0, 'offsetY': 0, 'zIndex': z}
    base['chartConfig']['title'] = title or base['chartConfig']['title']
    base['events'] = {'baseEvent': {}, 'advancedEvents': {}, 'interactEvents': []}
    base['filter'] = ''
    base['preview'] = {'overFlowHidden': False}
    base['request'] = {'requestDataType': 0, 'requestUrl': '',
                       'requestParams': {'Header': {}, 'Params': {}, 'Body': {}}}
    base.pop('key', None)
    if options:
        deep_merge(base['option'], options)
    return base


def deep_merge(target, patch):
    for key, value in patch.items():
        current = target.get(key)
        if isinstance(value, dict) and isinstance(current, dict):
            deep_merge(current, value)
        elif (isinstance(value, list) and isinstance(current, list) and value
              and all(isinstance(item, dict) for item in value)):
            # Chart options hold one template per series. Merging positionally
            # lets the screen switch a single property (a data label) without
            # discarding the gradient fill the template carries.
            for index, item in enumerate(value):
                if index < len(current) and isinstance(current[index], dict):
                    deep_merge(current[index], item)
                elif index < len(current):
                    current[index] = item
        else:
            target[key] = value


def number_option(suffix, color, size=40):
    return {'dataset': 0, 'from': 0, 'dur': 1.2, 'precision': 0, 'showSeparator': True,
            'numberSize': size, 'numberColor': color,
            'prefixText': '', 'prefixColor': color,
            'suffixText': suffix, 'suffixColor': '#7a93ad'}


def pie_option(center_text, size=34):
    return {'title': {'text': center_text, 'x': 'center', 'y': 'center',
                      'textStyle': {'color': '#00d4ff', 'fontSize': size, 'fontWeight': 'bold'}}}


def axis_titles(x_name, y_name):
    return {'xAxis': {'name': x_name}, 'yAxis': {'name': y_name}}


def no_labels():
    """240 trend points cannot carry a value label each: keep the line only."""
    return {'series': [{'label': {'show': False}}, {'label': {'show': False}}]}


def build():
    defaults = load_defaults()
    components = []

    # Background frame first: drawing order is list order.
    components.append(component(defaults, 'Border13', 'id_frame', 0, 0, 1920, 1080,
                                options={'backgroundColor': 'rgba(5,11,22,0)'}))
    components.append(component(defaults, 'TextGradient', 'id_title', 600, 16, 720, 60, options={
        'dataset': '消防设施物联网监控中心', 'size': 42,
        'gradient': {'type': 'linear', 'x': 0, 'y': 0, 'x2': 1, 'y2': 0,
                     'colorStops': [{'offset': 0, 'color': '#00e5ff'}, {'offset': 1, 'color': '#2f7bff'}]},
        'fontColor': '#00e5ff'}))
    components.append(component(defaults, 'TimeCommon', 'id_clock', 1608, 14, 288, 64,
                                options={'timeSize': 22, 'timeLineHeight': 30, 'timeColor': '#8fd6ff'}))

    # Left rail: fire alarm host counters.
    counters = [
        ('id_kpi_fire', 24, 104, '火警', ' 起', '#ff4d4f', 'id_lbl_fire'),
        ('id_kpi_fault', 254, 104, '故障', ' 处', '#ffa940', 'id_lbl_fault'),
        ('id_kpi_super', 24, 246, '监管', ' 条', '#ffd666', 'id_lbl_super'),
        ('id_kpi_shield', 254, 246, '隔离', ' 点', '#9254de', 'id_lbl_shield'),
    ]
    for cid, x, y, label, suffix, color, label_id in counters:
        components.append(component(defaults, 'TextCommon', label_id, x, y, 214, 28, options={
            'dataset': label, 'fontSize': 15, 'fontColor': '#7fa6c4'}))
        components.append(component(defaults, 'Number', cid, x, y + 30, 214, 96, title=label,
                                    options=number_option(suffix, color)))

    components.append(component(defaults, 'TextCommon', 'id_lbl_battery', 24, 384, 300, 22, options={
        'dataset': '集中电源蓄电池剩余电量', 'fontSize': 14, 'fontColor': '#7fa6c4', 'textAlign': 'left'}))
    components.append(component(defaults, 'PieCircle', 'id_battery', 24, 410, 444, 322,
                                title='集中电源蓄电池', options=pie_option('-- %')))
    components.append(component(defaults, 'TextCommon', 'id_note', 24, 744, 444, 312, options={
        'dataset': wrap('数据口径：接入消防设施 10 类。火警 / 故障 / 监管 / 隔离取自火灾报警主机，'
                        '蓄电池容量、管网压力、水池液位为实时量测。离线设备显示“暂无数据”，不做补值。'
                        '趋势图取近 30 天历史，按均值聚合到 240 点；无数据时段留空，不插值。', 22),
        'fontSize': 16, 'fontColor': '#8fb3cc', 'textAlign': 'left', 'paddingX': 4, 'paddingY': 0}))

    # Centre: trends, loop leakage, pump pressure, facility map.
    def caption(cid, x, y, w, text):
        components.append(component(defaults, 'TextCommon', cid, x, y, w, 22, options={
            'dataset': text, 'fontSize': 14, 'fontColor': '#7fa6c4', 'textAlign': 'left'}))

    caption('id_lbl_water', 484, 96, 320, '消防水位趋势 · 近 30 天')
    components.append(component(defaults, 'LineGradients', 'id_water', 484, 120, 952, 274,
                                title='消防水位趋势',
                                options=dict(axis_titles('时间', '液位 (m)'), **no_labels())))
    caption('id_lbl_leak', 484, 400, 320, '四回路剩余漏电电流 · 近 30 天')
    components.append(component(defaults, 'LineCommon', 'id_leak', 484, 424, 468, 276,
                                title='四回路剩余漏电电流',
                                options=dict(axis_titles('时间', '电流 (mA)'), **no_labels())))
    caption('id_lbl_pressure', 968, 400, 320, '泵房出水总管水压')
    components.append(component(defaults, 'Dial', 'id_pressure', 968, 424, 468, 276, title='泵房出水总管水压'))
    components.append(component(defaults, 'LocationMap', 'id_map', 484, 716, 952, 340,
                                options={'coordinateSystem': 'wgs84', 'dataset': [], 'geoLines': [],
                                         # auto: 跟随控制台已配置的天地图 Key；缺 Key 时明确回退示意网格。
                                         'basemap': 'auto', 'dim': 0.55, 'maxZoom': 15}))

    # Right rail: live alarms and gas concentration.
    components.append(component(defaults, 'TableScrollBoard', 'id_alarms', 1452, 104, 444, 440,
                                title='实时告警 · 全域设备', options={
                                    'header': ['时间', '设备', '告警内容', '级别'],
                                    'columnWidth': [88, 96, 188, 56],
                                    'align': ['left', 'left', 'left', 'center'],
                                    'rowNum': 8, 'waitTime': 2.5, 'index': False,
                                    'headerBGC': '#0c2b45', 'oddRowBGC': '#06131f', 'evenRowBGC': '#08192a'}))
    caption('id_lbl_gas', 1452, 548, 300, '锅炉房可燃气体浓度 (%LEL)')
    components.append(component(defaults, 'PieCircle', 'id_gas', 1452, 572, 444, 288,
                                title='可燃气体浓度', options=pie_option('-- %LEL')))
    components.append(component(defaults, 'TextCommon', 'id_legend', 1452, 876, 444, 180, options={
        'dataset': wrap('状态说明：● 正常  ● 数据过期  ● 设备离线  ● 暂无数据。'
                        '大屏只展示已授权的设备与点位，越权绑定在运行态显示“无权限”。', 24),
        'fontSize': 15, 'fontColor': '#6f8ea8', 'textAlign': 'left', 'paddingX': 4, 'paddingY': 0}))

    bindings = {}
    targets = []

    def bind(cid, binding_id, binding, target):
        bindings[binding_id] = binding
        targets.append({'componentId': cid, 'bindingId': binding_id, 'target': target})

    host = 'DEV_06_FAS_BACNET_GW'
    for cid, binding_id, key in (
        ('id_kpi_fire', 'bind_fire', 'AI_ALARM_COUNT'),
        ('id_kpi_fault', 'bind_fault', 'AI_FAULT_COUNT'),
        ('id_kpi_super', 'bind_super', 'AI_SUPERVISORY_COUNT'),
        ('id_kpi_shield', 'bind_shield', 'AI_SHIELD_COUNT'),
    ):
        bind(cid, binding_id, {'kind': 'point', 'deviceCode': host, 'key': key}, 'metric.value')

    bind('id_battery', 'bind_battery',
         {'kind': 'point', 'deviceCode': 'DEV_10_EMERG_LIGHT_POWER', 'key': 'AI_BATTERY_CAPACITY', 'unit': '%'},
         'metric.value')
    bind('id_gas', 'bind_gas',
         {'kind': 'point', 'deviceCode': 'DEV_09_GAS_DETECTOR', 'key': 'AI_GAS_CONC_CH1', 'unit': '%LEL'},
         'metric.value')
    bind('id_pressure', 'bind_pressure',
         {'kind': 'point', 'deviceCode': 'DEV_01_PUMP_ROOM_CTRL', 'key': 'AI_OUTLET_PRESSURE',
          'unit': 'MPa', 'decimals': 2},
         'metric.value')

    # History windows cover the last 7 days; charts stay one payload shape.
    bind('id_water', 'bind_water', {
        'kind': 'history', 'deviceCode': 'DEV_02_WATER_LEVEL',
        'keys': ['AI_POOL_UNDER_LEVEL', 'AI_TANK_ROOF_LEVEL'],
        'windowMs': 30 * 86400000, 'maxPoints': 240, 'aggregate': 'avg'
    }, 'chart.dataset')
    bind('id_leak', 'bind_leak', {
        'kind': 'history', 'deviceCode': 'DEV_04_ELEC_FIRE_DET',
        'keys': ['AI_L1_RESIDUAL_CURRENT', 'AI_L2_RESIDUAL_CURRENT',
                 'AI_L3_RESIDUAL_CURRENT', 'AI_L4_RESIDUAL_CURRENT'],
        'windowMs': 30 * 86400000, 'maxPoints': 240, 'aggregate': 'avg'
    }, 'chart.dataset')

    # Manual installation positions live in the device-location table, so the map
    # does not depend on telemetry being online. The GB28181 camera is part of the
    # site survey as well: it is the only device currently producing alarms, so the
    # alarm orbiter would be empty without it.
    mapped = ['DEV_01_PUMP_ROOM_CTRL', 'DEV_02_WATER_LEVEL', 'DEV_03_PIPE_PRESSURE',
              'DEV_04_ELEC_FIRE_DET', 'DEV_05_POWER_MONITOR', 'DEV_06_FAS_BACNET_GW',
              'DEV_07_EXH_FAN_CTRL', 'DEV_08_DOOR_MONITOR', 'DEV_09_GAS_DETECTOR',
              'DEV_10_EMERG_LIGHT_POWER', 'gvoixo7mzai7mn6zm']
    bind('id_map', 'bind_map', {
        'kind': 'location', 'deviceCodes': mapped,
        'locationSource': 'device', 'coordinateSystem': 'wgs84'
    }, 'map.points')

    bind('id_alarms', 'bind_alarms', {
        'kind': 'alarmList', 'deviceCodes': mapped, 'maxPoints': 50
    }, 'table.rows')

    return {
        'schemaVersion': 'noyo.screen/1',
        'engine': ENGINE,
        'canvas': CANVAS,
        'content': {
            'componentList': components,
            'editCanvasConfig': EDIT_CANVAS,
            'requestGlobalConfig': {'requestOriginUrl': '', 'requestDataPond': [],
                                    'requestParams': {'Header': {}, 'Params': {}, 'Body': {}}},
        },
        'bindings': bindings,
        'targets': targets,
        'themeRefs': {},
    }


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--check', action='store_true', help='validate with the Go document validator')
    args = parser.parse_args()

    document = build()
    OUT.parent.mkdir(parents=True, exist_ok=True)
    OUT.write_text(json.dumps(document, ensure_ascii=False, indent=1), encoding='utf-8')
    print(f'components={len(document["content"]["componentList"])} '
          f'bindings={len(document["bindings"])} bytes={OUT.stat().st_size} -> {OUT}')

    if args.check:
        result = subprocess.run(['go', 'run', './tmp-screen-check', str(OUT)], cwd=ROOT / 'backend',
                                capture_output=True, text=True)
        print(result.stdout.strip() or result.stderr.strip())
        if result.returncode != 0:
            sys.exit(1)


if __name__ == '__main__':
    main()
