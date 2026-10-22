<template>
  <div class="go-canvas-setting">
    <n-form class="canvas-size-fields" size="small" label-placement="top" :show-feedback="false">
      <n-form-item :label="t('宽度', 'Width')">
        <!-- 尺寸选择 -->
        <n-input-number
          size="small"
          v-model:value="canvasConfig.width"
          :disabled="editCanvas.lockScale"
          :validator="validator"
          @update:value="changeSizeHandle"
        ></n-input-number>
      </n-form-item>
      <n-form-item :label="t('高度', 'Height')">
        <n-input-number
          size="small"
          v-model:value="canvasConfig.height"
          :disabled="editCanvas.lockScale"
          :validator="validator"
          @update:value="changeSizeHandle"
        ></n-input-number>
      </n-form-item>
    </n-form>

    <div class="upload-box">
      <n-upload
        v-model:file-list="uploadFileListRef"
        :show-file-list="false"
        :customRequest="customRequest"
        :onBeforeUpload="beforeUploadHandle"
      >
        <n-upload-dragger>
          <img v-if="canvasConfig.backgroundImage" class="upload-show" :src="canvasConfig.backgroundImage" :alt="t('背景', 'Background')" />
          <div class="upload-img" v-show="!canvasConfig.backgroundImage">
            <img src="@/assets/images/canvas/noImage.png" alt="" />
            <n-text class="upload-desc" depth="3">
              {{ t(`背景图需小于 ${backgroundImageSize} MB，支持 PNG/JPG/GIF`, `PNG/JPG/GIF background, up to ${backgroundImageSize} MB`) }}
            </n-text>
          </div>
        </n-upload-dragger>
      </n-upload>
    </div>
    <div class="canvas-page-fields">
      <div class="canvas-page-field">
        <n-text>{{ t('背景颜色', 'Background color') }}</n-text>
        <div class="picker-height">
          <n-color-picker
            v-if="!switchSelectColorLoading"
            size="small"
            style="width: 100%"
            v-model:value="canvasConfig.background"
            :showPreview="true"
            :swatches="swatchesColors"
          ></n-color-picker>
        </div>
      </div>
      <div class="canvas-page-field">
        <n-text>{{ t('应用类型', 'Background mode') }}</n-text>
        <n-select
          size="small"
          style="width: 100%"
          v-model:value="selectColorValue"
          :disabled="!canvasConfig.backgroundImage"
          :options="selectColorOptions"
          @update:value="selectColorValueHandle"
        />
      </div>
      <div class="canvas-page-field">
        <n-text>{{ t('背景控制', 'Background controls') }}</n-text>
        <div class="canvas-background-actions">
        <n-button class="clear-btn" size="small" :disabled="!canvasConfig.backgroundImage" @click="clearImage">
          {{ t('清除背景', 'Clear image') }}
        </n-button>
        <n-button class="clear-btn" size="small" :disabled="!canvasConfig.background" @click="clearColor">
          {{ t('清除颜色', 'Clear color') }}
        </n-button>
        </div>
      </div>
      <div class="canvas-page-field">
        <n-text>{{ t('适配方式', 'Scaling') }}</n-text>
        <n-button-group>
          <n-button
            v-for="item in previewTypeList"
            :key="item.key"
            :type="canvasConfig.previewScaleType === item.key ? 'primary' : 'tertiary'"
            ghost
            size="small"
            :aria-label="item.title"
            @click="selectPreviewType(item.key)"
          >
            <n-tooltip :show-arrow="false" trigger="hover">
              <template #trigger>
                <n-icon class="select-preview-icon" size="18">
                  <component :is="item.icon"></component>
                </n-icon>
              </template>
              {{ item.desc }}
            </n-tooltip>
          </n-button>
        </n-button-group>
      </div>
    </div>

    <!-- 滤镜 -->
    <styles-setting :isCanvas="true" :chartStyles="canvasConfig"></styles-setting>
    <n-divider style="margin: 16px 0"></n-divider>

    <!-- 主题选择和全局配置 -->
    <n-tabs class="tabs-box" size="small" type="segment">
      <n-tab-pane
        v-for="item in globalTabList"
        :key="item.key"
        :name="item.key"
        size="small"
        display-directive="show:lazy"
      >
        <template #tab>
          <n-space>
            <span>{{ item.title }}</span>
            <n-icon size="16" class="icon-position">
              <component :is="item.icon"></component>
            </n-icon>
          </n-space>
        </template>
        <component :is="item.render"></component>
      </n-tab-pane>
    </n-tabs>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, nextTick, watch } from 'vue'
import { state } from '@/noyo/bridge'
import { backgroundImageSize } from '@/settings/designSetting'
import { swatchesColors } from '@/settings/chartThemes/index'
import { FileTypeEnum } from '@/enums/fileTypeEnum'
import { useChartEditStore } from '@/store/modules/chartEditStore/chartEditStore'
import { EditCanvasConfigEnum } from '@/store/modules/chartEditStore/chartEditStore.d'
import { useSystemStore } from '@/store/modules/systemStore/systemStore'
import { StylesSetting } from '@/components/Pages/ChartItemSetting'
import { UploadCustomRequestOptions } from 'naive-ui'
import { loadAsyncComponent, fetchRouteParamsLocation } from '@/utils'
import { PreviewScaleEnum } from '@/enums/styleEnum'
import { ResultEnum } from '@/enums/httpEnum'
import { icon } from '@/plugins'
import { uploadFile } from '@/api/path'

const { ColorPaletteIcon } = icon.ionicons5
const { ScaleIcon, FitToScreenIcon, FitToHeightIcon, FitToWidthIcon } = icon.carbon
const t = (zh: string, en: string) => state.locale.startsWith('en') ? en : zh

const chartEditStore = useChartEditStore()
const systemStore = useSystemStore()
const canvasConfig = chartEditStore.getEditCanvasConfig
const editCanvas = chartEditStore.getEditCanvas

const uploadFileListRef = ref()
const switchSelectColorLoading = ref(false)
const selectColorValue = ref(0)

const ChartThemeColor = loadAsyncComponent(() => import('./components/ChartThemeColor/index.vue'))
const VChartThemeColor = loadAsyncComponent(() => import('./components/VChartThemeColor/index.vue'))

// 默认应用类型
const selectColorOptions = computed(() => [
  {
    label: t('应用颜色', 'Color'),
    value: 0
  },
  {
    label: t('应用背景', 'Image'),
    value: 1
  }
])

const globalTabList = computed(() => [
  {
    key: 'ChartTheme',
    title: t('默认主题', 'Chart theme'),
    icon: ColorPaletteIcon,
    render: ChartThemeColor
  },
  {
    key: 'VChartTheme',
    title: t('VChart 主题', 'VChart theme'),
    icon: ColorPaletteIcon,
    render: VChartThemeColor
  }
])

const previewTypeList = computed(() => [
  {
    key: PreviewScaleEnum.FIT,
    title: t('自适应', 'Fit'),
    icon: ScaleIcon,
    desc: t('保持比例，页面可能留白', 'Fit proportionally with possible empty margins')
  },
  {
    key: PreviewScaleEnum.SCROLL_Y,
    title: t('Y 轴滚动', 'Fit width'),
    icon: FitToWidthIcon,
    desc: t('横向铺满，纵向滚动', 'Fit width and scroll vertically')
  },
  {
    key: PreviewScaleEnum.SCROLL_X,
    title: t('X 轴滚动', 'Fit height'),
    icon: FitToHeightIcon,
    desc: t('纵向铺满，横向滚动', 'Fit height and scroll horizontally')
  },
  {
    key: PreviewScaleEnum.FULL,
    title: t('铺满', 'Stretch'),
    icon: FitToScreenIcon,
    desc: t('拉伸画面，填满视图', 'Stretch to fill the view')
  }
])

watch(
  () => canvasConfig.selectColor,
  newValue => {
    selectColorValue.value = newValue ? 0 : 1
  },
  {
    immediate: true
  }
)

// 画布尺寸规则
const validator = (x: number) => x > 50

// 修改尺寸
const changeSizeHandle = () => {
  chartEditStore.computedScale()
}

// 上传图片前置处理
//@ts-ignore
const beforeUploadHandle = async ({ file }) => {
  uploadFileListRef.value = []
  const type = file.file.type
  const size = file.file.size

  if (size > 1024 * 1024 * backgroundImageSize) {
    window['$message'].warning(`图片超出 ${backgroundImageSize}M 限制，请重新上传！`)
    return false
  }
  if (type !== FileTypeEnum.PNG && type !== FileTypeEnum.JPEG && type !== FileTypeEnum.GIF) {
    window['$message'].warning('文件格式不符合，请重新上传！')
    return false
  }
  return true
}

// 应用颜色
const selectColorValueHandle = (value: number) => {
  canvasConfig.selectColor = value == 0
}

// 清除背景
const clearImage = () => {
  chartEditStore.setEditCanvasConfig(EditCanvasConfigEnum.BACKGROUND_IMAGE, undefined)
  chartEditStore.setEditCanvasConfig(EditCanvasConfigEnum.SELECT_COLOR, true)
}

// 启用/关闭 颜色（强制更新）
const switchSelectColorHandle = () => {
  switchSelectColorLoading.value = true
  setTimeout(() => {
    switchSelectColorLoading.value = false
  })
}

// 清除颜色
const clearColor = () => {
  chartEditStore.setEditCanvasConfig(EditCanvasConfigEnum.BACKGROUND, undefined)
  if (canvasConfig.backgroundImage) {
    chartEditStore.setEditCanvasConfig(EditCanvasConfigEnum.SELECT_COLOR, false)
  }
  switchSelectColorHandle()
}

// 自定义上传操作
const customRequest = (options: UploadCustomRequestOptions) => {
  const { file } = options
  nextTick(async () => {
    if (file.file) {
      // 修改名称
      const newNameFile = new File([file.file], `${fetchRouteParamsLocation()}_index_background.png`, {
        type: file.file.type
      })
      let uploadParams = new FormData()
      uploadParams.append('object', newNameFile)
      window['$message'].warn('该上传接口与项目预览图为同一个，请自行在后端新增一个上传背景图接口')
      const uploadRes = await uploadFile(uploadParams)

      if (uploadRes && uploadRes.code === ResultEnum.SUCCESS) {
        if (uploadRes.data.fileurl) {
          chartEditStore.setEditCanvasConfig(
            EditCanvasConfigEnum.BACKGROUND_IMAGE,
            `${uploadRes.data.fileurl}?time=${new Date().getTime()}`
          )
        } else {
          chartEditStore.setEditCanvasConfig(
            EditCanvasConfigEnum.BACKGROUND_IMAGE,
            `${systemStore.getFetchInfo.OSSUrl || ''}${uploadRes.data.fileName}?time=${new Date().getTime()}`
          )
        }
        chartEditStore.setEditCanvasConfig(EditCanvasConfigEnum.SELECT_COLOR, false)
        return
      }
      window['$message'].error('添加图片失败，请稍后重试！')
    } else {
      window['$message'].error('添加图片失败，请稍后重试！')
    }
  })
}

// 选择适配方式
const selectPreviewType = (key: PreviewScaleEnum) => {
  chartEditStore.setEditCanvasConfig(EditCanvasConfigEnum.PREVIEW_SCALE_TYPE, key)
}
</script>

<style lang="scss" scoped>
$uploadHeight: 192px;
@include go(canvas-setting) {
  min-width: 0;
  padding-top: 16px;
  .canvas-size-fields,
  .canvas-background-actions {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 8px;
  }
  .canvas-size-fields {
    margin-bottom: 16px;
    :deep(.n-form-item) { min-width: 0; }
    :deep(.n-form-item-label) { padding-bottom: 8px; }
    :deep(.n-input-number) { width: 100%; }
  }
  .canvas-page-fields { display: grid; gap: 16px; }
  .canvas-page-field { display: grid; gap: 8px; min-width: 0; }
  .upload-box {
    cursor: pointer;
    min-width: 0;
    margin-bottom: 16px;
    @include deep() {
      .n-upload-trigger { display: block; width: 100%; }
      .n-upload-dragger {
        padding: 16px;
        width: 100%;
        background-color: var(--bg-body);
      }
    }
    .upload-show {
      width: 100%;
      height: $uploadHeight;
      object-fit: contain;
      border-radius: 8px;
    }
    .upload-img {
      display: flex;
      flex-direction: column;
      align-items: center;
      img {
        max-width: 100%;
        height: 128px;
        object-fit: contain;
      }
      .upload-desc {
        padding: 8px 0 0;
        overflow-wrap: anywhere;
      }
    }
  }
  .icon-position {
    padding-top: 2px;
  }
  .picker-height {
    min-width: 0;
    min-height: 32px;
  }
  .clear-btn {
    width: 100%;
  }
  .select-preview-icon {
    padding: 0;
  }
  .tabs-box {
    margin-top: 16px;
  }
}
</style>
