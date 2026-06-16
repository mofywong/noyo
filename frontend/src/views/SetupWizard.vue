<template>
  <div class="setup-page" :class="`setup-mode-${form.mode}`">
    <div class="setup-backdrop" aria-hidden="true">
      <div class="backdrop-grid"></div>
      <div class="backdrop-panel panel-a"></div>
      <div class="backdrop-panel panel-b"></div>
    </div>

    <header class="setup-topbar">
      <div class="setup-brand">
        <div class="setup-logo-shell">
          <img src="/Noyo.svg" alt="Noyo" class="setup-logo-img">
        </div>
        <div>
          <div class="setup-brand-name">Noyo</div>
          <div class="setup-brand-subtitle">{{ tx('brandSubtitle') }}</div>
        </div>
      </div>

      <div class="setup-language" aria-label="Language">
        <button
          v-for="item in setupLanguageOptions"
          :key="item.value"
          type="button"
          class="language-btn"
          :class="{ active: currentLang === item.value }"
          @click="setSetupLanguage(item.value)"
        >
          {{ item.label }}
        </button>
      </div>
    </header>

    <main class="setup-shell">
      <aside class="setup-rail">
        <div class="rail-heading">
          <span class="rail-kicker">{{ tx('wizardKicker') }}</span>
          <h1>{{ tx('title') }}</h1>
          <p>{{ tx('subtitle') }}</p>
        </div>

        <nav class="setup-steps" aria-label="Setup sections">
          <button
            v-for="(step, index) in steps"
            :key="step.id"
            type="button"
            class="setup-step"
            :class="{ active: activeStep === step.id, complete: index < activeStepIndex }"
            @click="activeStep = step.id"
          >
            <span class="step-index">{{ index + 1 }}</span>
            <span>
              <strong>{{ step.label }}</strong>
              <small>{{ step.caption }}</small>
            </span>
          </button>
        </nav>

        <div class="setup-summary">
          <span>{{ tx('currentMode') }}</span>
          <strong>{{ currentModeLabel }}</strong>
          <small>{{ currentModeDescription }}</small>
        </div>
      </aside>

      <section class="setup-workspace">
        <div class="workspace-head">
          <div>
            <span class="workspace-eyebrow">{{ tx('deploymentProfile') }}</span>
            <h2>{{ activeStepTitle }}</h2>
          </div>
          <div class="setup-mode-badge">
            <i :class="currentModeIcon"></i>
            <span>{{ currentModeLabel }}</span>
          </div>
        </div>

        <div v-if="loadingStatus" class="setup-loading">
          <span class="spinner-border spinner-border-sm"></span>
          <span>{{ tx('loading') }}</span>
        </div>

        <div v-if="errorMsg" class="setup-alert error">
          <i class="bi bi-exclamation-triangle-fill"></i>
          <span>{{ errorMsg }}</span>
        </div>

        <div v-if="successMsg" class="setup-alert success">
          <i class="bi bi-check-circle-fill"></i>
          <span>{{ successMsg }}</span>
        </div>

        <form v-if="!loadingStatus" class="setup-form" novalidate @submit.prevent="submitSetup">
          <div class="setup-panel-window">
            <section v-if="activeStep === 'runtime'" class="setup-panel">
              <div class="setup-panel-head">
                <div>
                  <h3>{{ tx('runtimeTitle') }}</h3>
                  <p>{{ tx('runtimeDesc') }}</p>
                </div>
              </div>

              <div class="mode-grid">
                <label
                  v-for="mode in runtimeModeCards"
                  :key="mode.value"
                  class="mode-option"
                  :class="{ selected: form.mode === mode.value }"
                >
                  <input v-model="form.mode" type="radio" name="setup-mode" :value="mode.value">
                  <span :class="mode.visualClass" aria-hidden="true">
                  </span>
                  <span class="mode-copy">
                    <strong>{{ mode.label }}</strong>
                    <small>{{ mode.description }}</small>
                  </span>
                  <span class="mode-bullets">
                    <span v-for="bullet in mode.bullets" :key="bullet">{{ bullet }}</span>
                  </span>
                </label>
              </div>

              <div class="form-grid compact-grid">
                <label class="field-block">
                  <span>{{ tx('httpPort') }}</span>
                  <input v-model.number="form.server.port" type="number" min="1" max="65535" class="setup-input">
                </label>
              </div>
            </section>

            <section v-if="activeStep === 'admin'" class="setup-panel">
              <div class="setup-panel-head">
                <div>
                  <h3>{{ tx('adminTitle') }}</h3>
                  <p>{{ tx('adminDesc') }}</p>
                </div>
              </div>

              <div class="form-grid">
                <label class="field-block">
                  <span>{{ tx('username') }}</span>
                  <input v-model.trim="form.admin.username" type="text" class="setup-input" autocomplete="username">
                </label>
                <label class="field-block">
                  <span>{{ tx('displayName') }}</span>
                  <input v-model.trim="form.admin.display_name" type="text" class="setup-input">
                </label>
                <label class="field-block">
                  <span>{{ tx('password') }}</span>
                  <div class="input-group-custom">
                    <input v-model="form.admin.password" :type="showPassword ? 'text' : 'password'" class="setup-input" autocomplete="new-password">
                    <button type="button" class="password-toggle" @click="showPassword = !showPassword" tabindex="-1">
                      <i :class="showPassword ? 'bi bi-eye-slash' : 'bi bi-eye'"></i>
                    </button>
                  </div>
                </label>
                <label class="field-block">
                  <span>{{ tx('confirmPassword') }}</span>
                  <div class="input-group-custom">
                    <input v-model="confirmPassword" :type="showConfirmPassword ? 'text' : 'password'" class="setup-input" autocomplete="new-password">
                    <button type="button" class="password-toggle" @click="showConfirmPassword = !showConfirmPassword" tabindex="-1">
                      <i :class="showConfirmPassword ? 'bi bi-eye-slash' : 'bi bi-eye'"></i>
                    </button>
                  </div>
                </label>
              </div>
            </section>

            <section v-if="activeStep === 'project'" class="setup-panel">
              <div class="setup-panel-head">
                <div>
                  <h3>{{ tx('projectTitle') }}</h3>
                  <p>{{ modeNeedsProject ? tx('projectGatewayDesc') : tx('projectPlatformDesc') }}</p>
                </div>
              </div>

              <div v-if="modeNeedsProject" class="form-grid">
                <label class="field-block">
                  <span>{{ tx('projectName') }}</span>
                  <input v-model.trim="form.local_project.project_name" type="text" class="setup-input">
                </label>
              </div>

              <div v-else class="setup-empty-state">
                <i class="bi bi-building"></i>
                <span>{{ tx('projectPlatformEmpty') }}</span>
              </div>
            </section>

            <section v-if="activeStep === 'plugins'" class="setup-panel">
              <div class="setup-panel-head">
                <div>
                  <h3>{{ tx('pluginsTitle') }}</h3>
                  <p>{{ tx('pluginsDesc') }}</p>
                </div>
              </div>

              <div v-if="pluginSteps.length === 0" class="setup-empty-state">
                <i class="bi bi-check2-circle"></i>
                <span>{{ tx('pluginsEmpty') }}</span>
              </div>

              <div v-else class="plugin-setup-list">
                <article v-for="plugin in pluginSteps" :key="plugin.plugin_name" class="plugin-setup-block">
                  <div class="plugin-setup-title">
                    <div>
                      <h4>{{ localized(plugin.title) || plugin.plugin_name }}</h4>
                      <p v-if="localized(plugin.description)">{{ localized(plugin.description) }}</p>
                    </div>
                    <span>{{ plugin.plugin_name }}</span>
                  </div>

                  <div class="form-grid">
                    <div
                      v-for="field in visiblePluginFields(plugin)"
                      :key="`${plugin.plugin_name}-${field.name}`"
                      class="field-block"
                    >
                      <template v-if="field.type === 'switch'">
                        <label class="setup-switch">
                          <input
                            v-model="pluginForms[plugin.plugin_name][field.name]"
                            type="checkbox"
                          >
                          <span>{{ localized(field.title) || field.name }}</span>
                        </label>
                      </template>

                      <template v-else>
                        <label :for="fieldId(plugin.plugin_name, field.name)">
                          {{ cascadeFieldTitle(plugin, field) }}
                          <b v-if="field.required">*</b>
                        </label>
                        <select
                          v-if="field.type === 'select'"
                          v-model="pluginForms[plugin.plugin_name][field.name]"
                          class="setup-input"
                          :id="fieldId(plugin.plugin_name, field.name)"
                        >
                          <option v-for="option in field.options || []" :key="option.value" :value="option.value">
                            {{ option.label }}
                          </option>
                        </select>
                        <input
                          v-else
                          v-model.trim="pluginForms[plugin.plugin_name][field.name]"
                          class="setup-input"
                          :id="fieldId(plugin.plugin_name, field.name)"
                          :type="field.type === 'password' ? 'password' : 'text'"
                        >
                        <small v-if="cascadeFieldDescription(plugin, field)">{{ cascadeFieldDescription(plugin, field) }}</small>
                      </template>
                    </div>
                  </div>
                </article>
              </div>
            </section>
          </div>

          <div class="setup-actions">
            <button
              type="button"
              class="setup-btn secondary"
              :disabled="activeStepIndex === 0 || submitting"
              @click="goPrev"
            >
              <i class="bi bi-arrow-left"></i>
              {{ tx('prev') }}
            </button>
            <button
              v-if="activeStepIndex < steps.length - 1"
              type="button"
              class="setup-btn primary"
              :disabled="submitting"
              @click="goNext"
            >
              {{ tx('next') }}
              <i class="bi bi-arrow-right"></i>
            </button>
            <button v-else type="submit" class="setup-btn success" :disabled="submitting">
              <span v-if="submitting" class="spinner-border spinner-border-sm"></span>
              <i v-else class="bi bi-check2"></i>
              {{ tx('finish') }}
            </button>
          </div>
        </form>
      </section>
    </main>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import axios from 'axios'
import { clearSetupStatusCache } from '../router'
import { useAuthStore } from '../stores/auth'
import { SYSTEM_MODES, isSingleProjectMode } from '../utils/systemMode'

const router = useRouter()
const authStore = useAuthStore()
const { locale } = useI18n()

const setupLanguageOptions = [
  { value: 'zh', label: '中文' },
  { value: 'en', label: 'English' },
]

const setupCopy = {
  zh: {
    brandSubtitle: '首次部署向导',
    wizardKicker: 'Deployment',
    title: '初始化配置',
    subtitle: '选择 Noyo 的部署用途，创建第一个管理员，并按需配置级联连接。',
    deploymentProfile: '部署配置',
    currentMode: '当前模式',
    runtime: '运行模式',
    runtimeCaption: '平台或网关',
    admin: '管理员',
    adminCaption: '首个账号',
    project: '项目',
    projectCaption: '本地边界',
    plugins: '级联',
    pluginsCaption: '连接参数',
    loading: '正在读取初始化状态...',
    runtimeTitle: '选择部署模式',
    runtimeDesc: '平台模式用于上级管理，网关和本地项目模式用于边缘侧或小项目现场。',
    httpPort: 'HTTP 服务端口',
    adminTitle: '管理员账号',
    adminDesc: '该账号会成为初始化完成后的第一个登录账号。',
    username: '用户名',
    displayName: '显示名称',
    password: '密码',
    confirmPassword: '确认密码',
    projectTitle: '项目边界',
    projectGatewayDesc: '该模式会初始化一个隐藏租户和一个本地项目，用于承载项目内权限与设备数据。',
    projectPlatformDesc: '该模式不固定本地项目，项目可在初始化后创建。',
    tenantName: '本地租户名称',
    projectName: '本地项目名称',
    projectPlatformEmpty: '平台模式不在初始化时固定项目，后续通过系统管理创建租户和项目。',
    pluginsTitle: '级联插件配置',
    pluginsDesc: '这里只配置平台与托管网关之间的级联 Broker。MQTT 接口插件属于对外 API 接入方式，不在首次部署中配置。',
    pluginsEmpty: '当前模式没有必须在初始化阶段配置的级联参数。',
    prev: '上一步',
    next: '下一步',
    finish: '完成初始化',
    multiTenantPlatformLabel: '多租户运营平台',
    multiTenantPlatformDesc: '面向平台运营或集团化场景，保留完整租户隔离与多项目 RBAC。',
    multiProjectPlatformLabel: '多项目管理平台',
    multiProjectPlatformDesc: '面向单一组织管理多个项目，页面隐藏租户但底层保留默认租户边界。',
    platformGatewayLabel: '平台接入网关',
    platformGatewayDesc: '边缘侧单项目网关，通过级联 Broker 接入上级平台。',
    localProjectLabel: '本地单项目管理',
    localProjectDesc: '小项目本地独立运行，不接平台，单点完成接入与设备管理。',
    multiTenantPlatformBullets: ['多租户', '多项目', '平台运营'],
    multiProjectPlatformBullets: ['单组织', '多项目', '隐藏租户'],
    platformGatewayBullets: ['单项目', '接入平台', '级联必填'],
    localProjectBullets: ['单项目', '本地自治', '不接平台'],
    validationPasswordMismatch: '两次输入的管理员密码不一致',
    validationPasswordRequired: '管理员密码不能为空',
    validationUsernameRequired: '管理员用户名不能为空',
    validationPort: 'HTTP 端口必须在 1 到 65535 之间',
    validationTenantName: '该模式需要填写默认组织名称',
    validationLocalScope: '该模式需要填写默认组织和本地项目名称',
    validationGatewaySN: '平台接入网关需要填写平台预登记网关 SN',
    validationCascadeMqtt: '平台接入网关需要填写级联 MQTT 地址',
    cascadeMqttTitle: '级联 MQTT 地址',
    cascadeMqttDesc: '平台与托管网关共用的级联 Broker，例如 tcp://127.0.0.1:1883。',
    platformGatewaySnTitle: '平台注册网关 SN',
    platformGatewaySnDesc: '必须与平台在目标项目下预登记的网关设备编码一致，平台据此完成项目绑定。',
    gatewayNameTitle: '网关显示名称',
    gatewayNameDesc: '仅作为注册上报时的显示名称。',
    loadFailed: '初始化状态读取失败',
    setupFailed: '初始化失败',
    setupCompleted: '初始化完成',
  },
  en: {
    brandSubtitle: 'First-run setup',
    wizardKicker: 'Deployment',
    title: 'Initial Setup',
    subtitle: 'Choose the deployment profile, create the first admin, and configure cascade connectivity when needed.',
    deploymentProfile: 'Deployment Profile',
    currentMode: 'Current Mode',
    runtime: 'Runtime',
    runtimeCaption: 'Platform or gateway',
    admin: 'Admin',
    adminCaption: 'First account',
    project: 'Project',
    projectCaption: 'Local boundary',
    plugins: 'Cascade',
    pluginsCaption: 'Connection',
    loading: 'Loading setup status...',
    runtimeTitle: 'Choose Runtime Mode',
    runtimeDesc: 'Platform modes manage upstream scopes. Gateway and local project modes run at the edge or a small site.',
    httpPort: 'HTTP Server Port',
    adminTitle: 'Administrator Account',
    adminDesc: 'This account becomes the first login account after setup.',
    username: 'Username',
    displayName: 'Display Name',
    password: 'Password',
    confirmPassword: 'Confirm Password',
    projectTitle: 'Project Boundary',
    projectGatewayDesc: 'This mode initializes one hidden tenant and one local project for project permissions and device data.',
    projectPlatformDesc: 'This mode does not pin a local project during setup. Projects can be created after setup.',
    tenantName: 'Local Tenant Name',
    projectName: 'Local Project Name',
    projectPlatformEmpty: 'Platform mode does not pin a project during setup. Create tenants and projects later in system management.',
    pluginsTitle: 'Cascade Plugin Setup',
    pluginsDesc: 'Only the cascade broker between the platform and managed gateways is configured here. The MQTT API plugin is an external API integration method and is configured later.',
    pluginsEmpty: 'This mode has no cascade settings required during first-run setup.',
    prev: 'Previous',
    next: 'Next',
    finish: 'Finish Setup',
    multiTenantPlatformLabel: 'Multi-Tenant Operations Platform',
    multiTenantPlatformDesc: 'For platform operators or group deployments that need tenant isolation and multi-project RBAC.',
    multiProjectPlatformLabel: 'Multi-Project Management Platform',
    multiProjectPlatformDesc: 'For one organization managing multiple projects. Tenant scope is hidden but kept internally.',
    platformGatewayLabel: 'Platform-Connected Gateway',
    platformGatewayDesc: 'Single-project edge gateway that connects to an upstream platform through cascade.',
    localProjectLabel: 'Local Single-Project Management',
    localProjectDesc: 'For small local projects that manage access and devices without connecting to a platform.',
    multiTenantPlatformBullets: ['Multi-tenant', 'Multi-project', 'Operations'],
    multiProjectPlatformBullets: ['Single organization', 'Multi-project', 'Tenant hidden'],
    platformGatewayBullets: ['Single project', 'Platform-connected', 'Cascade required'],
    localProjectBullets: ['Single project', 'Local control', 'No platform'],
    validationPasswordMismatch: 'The two administrator passwords do not match',
    validationPasswordRequired: 'Administrator password is required',
    validationUsernameRequired: 'Administrator username is required',
    validationPort: 'HTTP port must be between 1 and 65535',
    validationTenantName: 'This mode requires a default organization name',
    validationLocalScope: 'This mode requires default organization and local project names',
    validationGatewaySN: 'Platform-connected gateway requires the platform pre-registered gateway SN',
    validationCascadeMqtt: 'Platform-connected gateway requires the cascade MQTT URL',
    cascadeMqttTitle: 'Cascade MQTT URL',
    cascadeMqttDesc: 'Shared broker used by platform and managed gateways, e.g. tcp://127.0.0.1:1883.',
    platformGatewaySnTitle: 'Platform Gateway SN',
    platformGatewaySnDesc: 'Must match the gateway device code that the platform pre-registered under the target project.',
    gatewayNameTitle: 'Gateway Display Name',
    gatewayNameDesc: 'Used only as the reported display name during registration.',
    loadFailed: 'Failed to load setup status',
    setupFailed: 'Setup failed',
    setupCompleted: 'Setup completed',
  },
}

const loadingStatus = ref(true)
const submitting = ref(false)
const errorMsg = ref('')
const successMsg = ref('')
const activeStep = ref('runtime')
const confirmPassword = ref('')
const showPassword = ref(false)
const showConfirmPassword = ref(false)
const setupStatus = ref(null)
const pluginForms = reactive({})
const setupHiddenFieldNames = new Set(['mode'])

const form = reactive({
  mode: SYSTEM_MODES.MULTI_TENANT_PLATFORM,
  server: {
    port: 8999,
  },
  admin: {
    username: 'admin',
    display_name: '超级管理员',
    password: '',
  },
  local_project: {
    tenant_name: 'Default Organization',
    project_name: 'Default Project',
  },
})

const currentLang = computed(() => String(locale.value || 'zh').startsWith('en') ? 'en' : 'zh')
const tx = (key) => setupCopy[currentLang.value][key] || setupCopy.zh[key] || key

const steps = computed(() => [
  { id: 'runtime', label: tx('runtime'), caption: tx('runtimeCaption') },
  { id: 'admin', label: tx('admin'), caption: tx('adminCaption') },
  { id: 'project', label: tx('project'), caption: tx('projectCaption') },
  { id: 'plugins', label: tx('plugins'), caption: tx('pluginsCaption') },
])

const modeUi = computed(() => ({
  [SYSTEM_MODES.MULTI_TENANT_PLATFORM]: {
    label: tx('multiTenantPlatformLabel'),
    description: tx('multiTenantPlatformDesc'),
    bullets: setupCopy[currentLang.value].multiTenantPlatformBullets,
    icon: 'bi bi-cloud-check',
    visualClass: 'mode-visual platform-multi',
  },
  [SYSTEM_MODES.MULTI_PROJECT_PLATFORM]: {
    label: tx('multiProjectPlatformLabel'),
    description: tx('multiProjectPlatformDesc'),
    bullets: setupCopy[currentLang.value].multiProjectPlatformBullets,
    icon: 'bi bi-kanban',
    visualClass: 'mode-visual platform-single',
  },
  [SYSTEM_MODES.PLATFORM_GATEWAY]: {
    label: tx('platformGatewayLabel'),
    description: tx('platformGatewayDesc'),
    bullets: setupCopy[currentLang.value].platformGatewayBullets,
    icon: 'bi bi-diagram-3',
    visualClass: 'mode-visual gateway-managed',
  },
  [SYSTEM_MODES.LOCAL_PROJECT]: {
    label: tx('localProjectLabel'),
    description: tx('localProjectDesc'),
    bullets: setupCopy[currentLang.value].localProjectBullets,
    icon: 'bi bi-hdd-network',
    visualClass: 'mode-visual gateway-standalone',
  },
}))

const activeStepIndex = computed(() => steps.value.findIndex(step => step.id === activeStep.value))
const runtimeModes = computed(() => setupStatus.value?.runtime_modes || [])
const pluginSteps = computed(() => (setupStatus.value?.plugin_steps || []).filter(plugin => plugin.plugin_name === 'cascade'))
const modeNeedsProject = computed(() => isSingleProjectMode(form.mode))
const modeNeedsCascadeRegistration = computed(() => form.mode === SYSTEM_MODES.PLATFORM_GATEWAY)
const runtimeModeCards = computed(() => {
  const values = runtimeModes.value.length > 0
    ? runtimeModes.value.map(item => item.value)
    : [
        SYSTEM_MODES.MULTI_TENANT_PLATFORM,
        SYSTEM_MODES.MULTI_PROJECT_PLATFORM,
        SYSTEM_MODES.PLATFORM_GATEWAY,
        SYSTEM_MODES.LOCAL_PROJECT,
      ]
  return values.map(value => ({
    value,
    ...(modeUi.value[value] || {
      label: value,
      description: '',
      bullets: [],
      icon: 'bi bi-box',
      visualClass: 'mode-visual platform',
    }),
  }))
})

const currentMode = computed(() => runtimeModeCards.value.find(item => item.value === form.mode) || runtimeModeCards.value[0] || {})
const currentModeLabel = computed(() => currentMode.value.label || form.mode)
const currentModeDescription = computed(() => currentMode.value.description || '')
const currentModeIcon = computed(() => currentMode.value.icon || 'bi bi-box')
const activeStepTitle = computed(() => steps.value[activeStepIndex.value]?.label || '')

const setSetupLanguage = (lang) => {
  locale.value = lang
  localStorage.setItem('lang', lang)
}

const localized = (value) => {
  if (!value) return ''
  if (typeof value === 'string') return value
  return value[currentLang.value] || value.zh || value.en || Object.values(value)[0] || ''
}

const cascadeFieldTitle = (plugin, field) => {
  if (plugin?.plugin_name === 'cascade') {
    if (field.name === 'mqtt_url') return tx('cascadeMqttTitle')
    if (field.name === 'gateway_sn') return tx('platformGatewaySnTitle')
    if (field.name === 'gateway_name') return tx('gatewayNameTitle')
  }
  return localized(field.title) || field.name
}

const cascadeFieldDescription = (plugin, field) => {
  if (plugin?.plugin_name === 'cascade') {
    if (field.name === 'mqtt_url') return tx('cascadeMqttDesc')
    if (field.name === 'gateway_sn') return tx('platformGatewaySnDesc')
    if (field.name === 'gateway_name') return tx('gatewayNameDesc')
  }
  return localized(field.description)
}

const defaultFieldValue = (field) => {
  if (field.value !== undefined && field.value !== null) return field.value
  if (field.type === 'switch') return false
  if (field.type === 'select') return field.options?.[0]?.value || ''
  return ''
}

const visiblePluginFields = (plugin) => (plugin.fields || []).filter(field => !setupHiddenFieldNames.has(field.name))

const normalizePluginForms = () => {
  const activePluginNames = new Set(pluginSteps.value.map(plugin => plugin.plugin_name))
  for (const name of Object.keys(pluginForms)) {
    if (!activePluginNames.has(name)) {
      delete pluginForms[name]
    }
  }
  for (const plugin of pluginSteps.value) {
    if (!pluginForms[plugin.plugin_name]) {
      pluginForms[plugin.plugin_name] = {}
    }
    for (const field of visiblePluginFields(plugin)) {
      if (pluginForms[plugin.plugin_name][field.name] === undefined) {
        pluginForms[plugin.plugin_name][field.name] = defaultFieldValue(field)
      }
    }
  }
}

const loadSetupStatus = async () => {
  loadingStatus.value = true
  errorMsg.value = ''
  try {
    const res = await axios.get('/api/setup/status', { params: { mode: form.mode } })
    if (res.data.code !== 0) {
      throw new Error(res.data.message || tx('loadFailed'))
    }
    setupStatus.value = res.data.data
    if (setupStatus.value.initialized) {
      clearSetupStatusCache()
      await router.replace('/login')
      return
    }
    if (setupStatus.value.server_port) {
      form.server.port = setupStatus.value.server_port
    }
    normalizePluginForms()
  } catch (error) {
    errorMsg.value = error.response?.data?.message || error.message || tx('loadFailed')
  } finally {
    loadingStatus.value = false
  }
}

const fieldId = (pluginName, fieldName) => `setup-${pluginName}-${fieldName}`

const goPrev = () => {
  if (activeStepIndex.value > 0) {
    activeStep.value = steps.value[activeStepIndex.value - 1].id
  }
}

const goNext = () => {
  if (activeStepIndex.value < steps.value.length - 1) {
    activeStep.value = steps.value[activeStepIndex.value + 1].id
  }
}

const validateBeforeSubmit = () => {
  const port = Number(form.server.port)
  if (!Number.isInteger(port) || port < 1 || port > 65535) {
    activeStep.value = 'runtime'
    return tx('validationPort')
  }
  if (!form.admin.username.trim()) {
    activeStep.value = 'admin'
    return tx('validationUsernameRequired')
  }
  if (!form.admin.password) {
    activeStep.value = 'admin'
    return tx('validationPasswordRequired')
  }
  if (form.admin.password !== confirmPassword.value) {
    activeStep.value = 'admin'
    return tx('validationPasswordMismatch')
  }
  if (modeNeedsProject.value && !form.local_project.project_name.trim()) {
    activeStep.value = 'project'
    return tx('validationLocalScope')
  }
  if (modeNeedsCascadeRegistration.value) {
    const cascade = pluginForms.cascade || {}
    if (!String(cascade.gateway_sn || '').trim()) {
      activeStep.value = 'plugins'
      return tx('validationGatewaySN')
    }
    if (!String(cascade.mqtt_url || '').trim()) {
      activeStep.value = 'plugins'
      return tx('validationCascadeMqtt')
    }
  }
  return ''
}

const clonePluginForms = () => {
  const payload = {}
  for (const [name, values] of Object.entries(pluginForms)) {
    payload[name] = { ...values }
  }
  return payload
}

const submitSetup = async () => {
  errorMsg.value = ''
  successMsg.value = ''
  const validationError = validateBeforeSubmit()
  if (validationError) {
    errorMsg.value = validationError
    return
  }

  submitting.value = true
  try {
    const plugins = clonePluginForms()
    const cascade = plugins.cascade || {}
    const payload = {
      mode: form.mode,
      server: {
        port: Number(form.server.port),
      },
      admin: {
        username: form.admin.username,
        display_name: form.admin.display_name,
        password: form.admin.password,
      },
      local_project: {
        tenant_name: '',
        project_name: modeNeedsProject.value ? form.local_project.project_name : '',
      },
      gateway: {
        gateway_sn: cascade.gateway_sn || '',
        gateway_name: cascade.gateway_name || '',
        mqtt_url: cascade.mqtt_url || '',
        enable_tls: Boolean(cascade.enable_tls),
        insecure_skip_verify: Boolean(cascade.insecure_skip_verify),
        username: cascade.username || '',
        password: cascade.password || '',
      },
      plugins,
    }

    const oldPort = window.location.port || (window.location.protocol === 'https:' ? '443' : '80')
    const newPort = String(payload.server.port)
    const portChanged = oldPort !== newPort

    const res = await axios.post('/api/setup/apply', payload)
    if (res.data.code !== 0) {
      throw new Error(res.data.message || tx('setupFailed'))
    }

    clearSetupStatusCache()
    authStore.logout()
    localStorage.removeItem('current_tenant_id')

    if (portChanged) {
      successMsg.value = `${tx('setupCompleted')} 端口已变更为 ${newPort}，后台服务重启中，将在3秒后自动跳转...`
      setTimeout(() => {
        window.location.href = `${window.location.protocol}//${window.location.hostname}:${newPort}/login`
      }, 3000)
    } else {
      successMsg.value = res.data.message || tx('setupCompleted')
      await router.replace('/login')
    }
  } catch (error) {
    errorMsg.value = error.response?.data?.message || error.message || tx('setupFailed')
  } finally {
    submitting.value = false
  }
}

watch(() => form.mode, async () => {
  await loadSetupStatus()
})

onMounted(loadSetupStatus)
</script>

<style scoped>
.setup-page {
  min-height: 100vh;
  height: 100vh;
  background: #060a12;
  color: #e5edf7;
  position: relative;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.setup-backdrop {
  position: fixed;
  inset: 0;
  pointer-events: none;
  overflow: hidden;
}

.backdrop-grid {
  position: absolute;
  inset: 0;
  background-image:
    linear-gradient(rgba(101, 156, 225, 0.08) 1px, transparent 1px),
    linear-gradient(90deg, rgba(101, 156, 225, 0.08) 1px, transparent 1px);
  background-size: 44px 44px;
  mask-image: linear-gradient(to bottom, rgba(0, 0, 0, 0.75), transparent 78%);
}

.backdrop-panel {
  position: absolute;
  border: 1px solid rgba(88, 166, 255, 0.16);
  background:
    linear-gradient(135deg, rgba(23, 65, 122, 0.28), rgba(8, 18, 32, 0.14)),
    url('/Noyo.svg') center / 62% no-repeat;
  opacity: 0.08;
  transform: rotate(-10deg);
}

.panel-a {
  width: 520px;
  height: 520px;
  right: -180px;
  top: 90px;
}

.panel-b {
  width: 360px;
  height: 360px;
  left: -120px;
  bottom: -140px;
  transform: rotate(13deg);
}

.setup-topbar {
  position: relative;
  z-index: 2;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 22px 32px 12px;
}

.setup-brand {
  display: flex;
  align-items: center;
  gap: 12px;
}

.setup-logo-shell {
  width: 46px;
  height: 46px;
  border-radius: 8px;
  display: grid;
  place-items: center;
  background: rgba(255, 255, 255, 0.94);
  border: 1px solid rgba(255, 255, 255, 0.22);
  box-shadow: 0 16px 40px rgba(3, 16, 36, 0.35);
  overflow: hidden;
}

.setup-logo-img {
  width: 36px;
  height: 36px;
  object-fit: contain;
}

.setup-brand-name {
  font-size: 21px;
  font-weight: 760;
  line-height: 1;
}

.setup-brand-subtitle {
  color: #8da3bd;
  font-size: 13px;
  margin-top: 4px;
}

.setup-language {
  display: inline-flex;
  padding: 4px;
  border: 1px solid rgba(148, 163, 184, 0.22);
  border-radius: 8px;
  background: rgba(8, 15, 28, 0.72);
}

.language-btn {
  border: 0;
  color: #90a2b8;
  background: transparent;
  border-radius: 6px;
  padding: 7px 12px;
  font-weight: 650;
}

.language-btn.active {
  color: #07111f;
  background: #dbeafe;
}

.setup-shell {
  position: relative;
  z-index: 1;
  flex: 1;
  min-height: 0;
  overflow: hidden;
  display: grid;
  grid-template-columns: 320px minmax(0, 1fr);
  gap: 22px;
  padding: 18px 32px 32px;
}

.setup-rail,
.setup-workspace {
  border: 1px solid rgba(148, 163, 184, 0.18);
  border-radius: 8px;
  background: rgba(9, 16, 30, 0.78);
  box-shadow: 0 24px 80px rgba(0, 0, 0, 0.34);
  backdrop-filter: blur(18px);
}

.setup-rail {
  padding: 24px;
  display: flex;
  flex-direction: column;
  gap: 22px;
}

.rail-kicker,
.workspace-eyebrow {
  color: #61dafb;
  text-transform: uppercase;
  letter-spacing: 0;
  font-size: 12px;
  font-weight: 760;
}

.rail-heading h1 {
  margin: 8px 0 10px;
  font-size: 28px;
  line-height: 1.15;
}

.rail-heading p,
.setup-summary small,
.setup-panel-head p,
.plugin-setup-title p,
.field-block small {
  color: #92a5bd;
  line-height: 1.55;
  margin: 0;
}

.setup-steps {
  display: grid;
  gap: 10px;
}

.setup-step {
  border: 1px solid rgba(148, 163, 184, 0.14);
  border-radius: 8px;
  background: rgba(15, 23, 42, 0.46);
  color: #b7c6d8;
  display: grid;
  grid-template-columns: 34px 1fr;
  gap: 12px;
  align-items: center;
  padding: 12px;
  text-align: left;
}

.setup-step.active {
  border-color: rgba(96, 165, 250, 0.62);
  background: rgba(37, 99, 235, 0.18);
  color: #f8fbff;
}

.setup-step.complete {
  color: #93e4b6;
}

.step-index {
  width: 34px;
  height: 34px;
  border-radius: 8px;
  display: grid;
  place-items: center;
  background: rgba(148, 163, 184, 0.14);
  font-weight: 760;
}

.setup-step strong,
.setup-step small {
  display: block;
}

.setup-step small {
  margin-top: 2px;
  color: inherit;
  opacity: 0.68;
}

.setup-summary {
  margin-top: auto;
  border: 1px solid rgba(96, 165, 250, 0.18);
  border-radius: 8px;
  background: rgba(4, 12, 24, 0.5);
  padding: 16px;
  display: grid;
  gap: 6px;
}

.setup-summary span {
  color: #88a1bc;
  font-size: 13px;
}

.setup-summary strong {
  font-size: 16px;
}

.setup-workspace {
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.workspace-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 18px;
  padding: 24px 26px 0;
}

.workspace-head h2 {
  margin: 6px 0 0;
  font-size: 26px;
}

.setup-mode-badge {
  flex: 0 0 auto;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: #bfdbfe;
  border: 1px solid rgba(96, 165, 250, 0.26);
  background: rgba(37, 99, 235, 0.16);
  border-radius: 8px;
  padding: 9px 12px;
  font-weight: 700;
}

.setup-loading,
.setup-alert {
  margin: 18px 26px 0;
  display: flex;
  align-items: center;
  gap: 10px;
  border-radius: 8px;
  padding: 12px 14px;
}

.setup-loading {
  color: #b6c6db;
  background: rgba(148, 163, 184, 0.1);
}

.setup-alert.error {
  color: #fecaca;
  border: 1px solid rgba(248, 113, 113, 0.28);
  background: rgba(127, 29, 29, 0.32);
}

.setup-alert.success {
  color: #bbf7d0;
  border: 1px solid rgba(74, 222, 128, 0.26);
  background: rgba(20, 83, 45, 0.28);
}

.setup-form {
  min-height: 0;
  flex: 1;
  display: flex;
  flex-direction: column;
}

.setup-panel-window {
  min-height: 0;
  flex: 1;
  overflow: auto;
  padding: 22px 26px;
}

.setup-panel {
  display: grid;
  gap: 22px;
}

.setup-panel-head {
  display: flex;
  justify-content: space-between;
  gap: 16px;
}

.setup-panel-head h3,
.plugin-setup-title h4 {
  margin: 0 0 6px;
  font-size: 19px;
}

.mode-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 14px;
}

.mode-option {
  border: 1px solid rgba(148, 163, 184, 0.18);
  border-radius: 8px;
  background: rgba(15, 23, 42, 0.62);
  min-height: 260px;
  padding: 12px;
  cursor: pointer;
  display: grid;
  grid-template-rows: auto 1fr auto;
  gap: 12px;
}

.mode-option.selected {
  border-color: rgba(96, 165, 250, 0.78);
  box-shadow: inset 0 0 0 1px rgba(96, 165, 250, 0.38), 0 18px 50px rgba(37, 99, 235, 0.22);
  background: rgba(15, 35, 68, 0.72);
}

.mode-option input {
  position: absolute;
  opacity: 0;
}

.mode-visual {
  height: 90px;
  position: relative;
  overflow: hidden;
  border-radius: 8px;
  border: 1px solid rgba(255, 255, 255, 0.12);
  display: block;
  background-size: cover;
  background-position: center;
}

.mode-visual.platform-multi {
  background-image: url('/mode_multi_tenant.png');
}

.mode-visual.platform-single {
  background-image: url('/mode_multi_project.png');
}

.mode-visual.gateway-standalone {
  background-image: url('/mode_local_project.png');
}

.mode-visual.gateway-managed {
  background-image: url('/mode_edge_gateway.png');
}

.mode-copy {
  display: grid;
  gap: 7px;
}

.mode-copy strong {
  font-size: 17px;
}

.mode-copy small,
.mode-bullets span {
  color: #9fb0c5;
  line-height: 1.48;
}

.mode-bullets {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.mode-bullets span {
  border: 1px solid rgba(148, 163, 184, 0.18);
  border-radius: 8px;
  padding: 5px 8px;
  background: rgba(255, 255, 255, 0.05);
  font-size: 12px;
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}

.compact-grid {
  max-width: 360px;
}

.field-block {
  min-width: 0;
  display: grid;
  gap: 7px;
}

.field-block > span,
.field-block > label {
  color: #d7e3f2;
  font-weight: 680;
  font-size: 13px;
}

.field-block b {
  color: #fca5a5;
}

.setup-input {
  width: 100%;
  min-height: 44px;
  border: 1px solid rgba(148, 163, 184, 0.22);
  border-radius: 8px;
  background: rgba(4, 12, 24, 0.72);
  color: #f8fbff;
  padding: 9px 12px;
  outline: none;
}

.setup-input:focus {
  border-color: rgba(96, 165, 250, 0.82);
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.18);
}

.input-group-custom {
  position: relative;
  display: flex;
  align-items: center;
}

.input-group-custom .setup-input {
  padding-right: 44px;
}

.password-toggle {
  position: absolute;
  right: 6px;
  background: transparent;
  border: none;
  color: #9fb0c5;
  height: 32px;
  width: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.2s;
}

.password-toggle:hover {
  color: #d7e3f2;
  background: rgba(255, 255, 255, 0.08);
}

.plugin-setup-list {
  display: grid;
  gap: 16px;
}

.plugin-setup-block {
  border: 1px solid rgba(148, 163, 184, 0.18);
  border-radius: 8px;
  background: rgba(15, 23, 42, 0.5);
  padding: 18px;
}

.plugin-setup-title {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
  margin-bottom: 16px;
}

.plugin-setup-title span {
  color: #bfdbfe;
  background: rgba(37, 99, 235, 0.2);
  border-radius: 8px;
  padding: 5px 9px;
  font-size: 12px;
  font-weight: 700;
}

.setup-switch {
  border: 1px solid rgba(148, 163, 184, 0.2);
  border-radius: 8px;
  min-height: 44px;
  padding: 10px 12px;
  background: rgba(4, 12, 24, 0.56);
  display: flex;
  align-items: center;
  gap: 10px;
}

.setup-empty-state {
  min-height: 164px;
  display: grid;
  place-items: center;
  gap: 10px;
  color: #9fb0c5;
  border: 1px dashed rgba(148, 163, 184, 0.25);
  border-radius: 8px;
  text-align: center;
  padding: 24px;
  background: rgba(4, 12, 24, 0.46);
}

.setup-empty-state i {
  font-size: 30px;
  color: #7dd3fc;
}

.setup-actions {
  position: sticky;
  bottom: 0;
  z-index: 3;
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  padding: 14px 26px 20px;
  border-top: 1px solid rgba(148, 163, 184, 0.14);
  background: linear-gradient(180deg, rgba(9, 16, 30, 0.78), rgba(6, 10, 18, 0.96));
}

.setup-btn {
  border: 0;
  border-radius: 8px;
  min-height: 42px;
  padding: 0 16px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  font-weight: 760;
}

.setup-btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.setup-btn.secondary {
  color: #d7e3f2;
  background: rgba(148, 163, 184, 0.14);
}

.setup-btn.primary {
  color: #07111f;
  background: #93c5fd;
}

.setup-btn.success {
  color: #06140b;
  background: #86efac;
}

@media (max-width: 1080px) {
  .setup-shell {
    grid-template-columns: 1fr;
  }

  .setup-rail {
    gap: 16px;
  }

  .setup-steps {
    grid-template-columns: repeat(4, minmax(0, 1fr));
  }

  .setup-summary {
    margin-top: 0;
  }

  .mode-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 760px) {
  .setup-topbar {
    padding: 16px;
    align-items: flex-start;
    gap: 14px;
  }

  .setup-shell {
    padding: 10px 14px 16px;
  }

  .workspace-head,
  .setup-panel-window,
  .setup-actions {
    padding-left: 16px;
    padding-right: 16px;
  }

  .workspace-head,
  .setup-topbar,
  .setup-actions {
    flex-wrap: wrap;
  }

  .mode-grid,
  .form-grid,
  .setup-steps {
    grid-template-columns: 1fr;
  }

  .setup-actions {
    justify-content: stretch;
  }

  .setup-btn {
    flex: 1 1 auto;
  }
}
</style>
