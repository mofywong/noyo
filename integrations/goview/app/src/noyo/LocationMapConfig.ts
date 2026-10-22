import { PublicConfigClass } from '@/packages/public'
import { ChartFrameEnum, PackagesCategoryEnum } from '@/packages/index.d'

export default class LocationMapConfig extends PublicConfigClass {
  public key = 'LocationMap'
  public chartConfig = { key: 'LocationMap', chartKey: 'VLocationMap', conKey: 'VCLocationMap', title: '设备地图', category: 'Maps', categoryName: 'Maps', package: PackagesCategoryEnum.CHARTS, chartFrame: ChartFrameEnum.COMMON, image: '' }
  public option = {
    dataset: [],
    coordinateSystem: 'wgs84',
    geoLines: [],
    // auto = 跟随「设备地图」页的底图选择与天地图 Key，大屏不再单独配置一次。
    basemap: 'auto',
    // 暗色屏亮度衰减：只作用于瓦片画布，DOM 标注不受影响。
    dim: 0.55,
    maxZoom: 15,
    interactive: true,
  }
  constructor() { super(); this.attr.w = 640; this.attr.h = 400 }
}
