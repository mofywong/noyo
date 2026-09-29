<template>
  <div class="cascade-config-panel">
    <div v-if="loading" class="text-center py-4">
      <div class="spinner-border text-primary" role="status"></div>
    </div>
    
    <div v-else>
      <form @submit.prevent="save" autocomplete="off">
        <!-- Mode Selection -->
        <div class="card mb-4 border-0 shadow-sm">
          <div class="card-header bg-light fw-bold">
            <i class="bi bi-hdd-network me-2 text-primary"></i>{{ C.modeTitle }}
          </div>
          <div class="card-body">
            <div class="row">
              <div class="col-md-6">
                <div class="form-check form-check-inline border rounded p-3 w-100 text-center"
                     :class="{'border-primary bg-primary-subtle': config.mode === 'platform'}"
                     @click="config.mode = 'platform'" style="cursor: pointer;">
                  <input class="form-check-input float-none ms-0 mb-2 d-block mx-auto" type="radio" 
                         name="mode" value="platform" v-model="config.mode">
                  <label class="form-check-label fw-bold d-block">{{ C.modePlatform }}</label>
                  <small class="text-muted d-block mt-1">{{ C.modePlatformHint }}</small>
                </div>
              </div>
              <div class="col-md-6">
                <div class="form-check form-check-inline border rounded p-3 w-100 text-center"
                     :class="{'border-primary bg-primary-subtle': config.mode === 'gateway'}"
                     @click="config.mode = 'gateway'" style="cursor: pointer;">
                  <input class="form-check-input float-none ms-0 mb-2 d-block mx-auto" type="radio" 
                         name="mode" value="gateway" v-model="config.mode">
                  <label class="form-check-label fw-bold d-block">{{ C.modeGateway }}</label>
                  <small class="text-muted d-block mt-1">{{ C.modeGatewayHint }}</small>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- MQTT Config -->
        <div class="card mb-4 border-0 shadow-sm">
          <div class="card-header bg-light fw-bold">
            <i class="bi bi-server me-2 text-primary"></i>{{ C.mqttTitle }}
          </div>
          <div class="card-body">
            <div class="mb-3">
              <label class="form-label fw-bold small">{{ C.mqttUrl }} <span class="text-danger">*</span></label>
              <input type="text" class="form-control" v-model="config.mqtt_url" placeholder="tcp://127.0.0.1:1883" required>
              <div class="form-text">{{ C.mqttUrlHint }}</div>
            </div>
            <div class="row">
              <div class="col-md-6 mb-3">
                <label class="form-label fw-bold small">{{ C.username }}</label>
                <input type="text" class="form-control" v-model="config.username" autocomplete="new-password">
              </div>
              <div class="col-md-6 mb-3">
                <label class="form-label fw-bold small">{{ C.password }}</label>
                <input type="password" class="form-control" v-model="config.password" autocomplete="new-password">
              </div>
            </div>
            <div class="row">
              <div class="col-md-6 mb-3">
                <div class="form-check form-switch pt-2">
                  <input class="form-check-input" type="checkbox" id="cascadeEnableTls" v-model="config.enable_tls">
                  <label class="form-check-label fw-bold small" for="cascadeEnableTls">
                    {{ C.enableTls }}
                  </label>
                  <div class="form-text">{{ C.enableTlsHint }}</div>
                </div>
              </div>
              <div class="col-md-6 mb-3" v-if="config.enable_tls">
                <div class="form-check form-switch pt-2">
                  <input class="form-check-input" type="checkbox" id="cascadeSkipVerify" v-model="config.insecure_skip_verify">
                  <label class="form-check-label fw-bold small" for="cascadeSkipVerify">
                    {{ C.skipVerify }}
                  </label>
                  <div class="form-text">{{ C.skipVerifyHint }}</div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Gateway Config -->
        <div v-if="config.mode === 'gateway'" class="card mb-4 border-0 shadow-sm">
          <div class="card-header bg-light fw-bold">
            <i class="bi bi-router me-2 text-success"></i>{{ C.gatewayTitle }}
          </div>
          <div class="card-body">
            <div class="mb-3">
              <label class="form-label fw-bold small">{{ C.gatewaySn }} <span class="text-danger">*</span></label>
              <input type="text" class="form-control" v-model="config.gateway_sn" :required="config.mode === 'gateway'">
              <div class="form-text">{{ C.gatewaySnHint }}</div>
            </div>
            <div class="mb-3">
              <label class="form-label fw-bold small">{{ C.gatewayName }}</label>
              <input type="text" class="form-control" v-model="config.gateway_name">
              <div class="form-text">{{ C.gatewayNameHint }}</div>
            </div>
          </div>
        </div>

        <!-- Actions -->
        <div class="d-flex justify-content-end pt-3 border-top gap-2">
          <button type="button" class="btn btn-light" @click="cancel">{{ C.cancel }}</button>
          <button type="submit" class="btn btn-primary px-4" :disabled="saving">
            <span v-if="saving" class="spinner-border spinner-border-sm me-2"></span>
            <i v-else class="bi bi-save me-2"></i>
            {{ saving ? C.saving : C.save }}
          </button>
        </div>
      </form>
    </div>
  </div>
</template>

<script setup>
import { computed, ref, onMounted } from 'vue';
import { useI18n } from 'vue-i18n';
import axios from 'axios';
import { useToast } from '@/composables/useToast';
import { useRouter } from 'vue-router';

const props = defineProps({
  pluginName: String,
  apiBase: String,
  remoteContext: Object
});
const emit = defineEmits(['saved', 'cancel']);
const { showToast } = useToast();
const router = useRouter();

// Copy follows the active console language. The panel previously hard-coded
// Chinese and printed both languages in one label ("运行模式 (Operating Mode)"),
// so an English console showed a Chinese form and neither console was
// single-language. Values the operator types stay untouched.
const { locale } = useI18n();
const isZh = computed(() => String(locale.value || '').toLowerCase().startsWith('zh'));
const C = computed(() => (isZh.value ? {
  modeTitle: '运行模式',
  modePlatform: '平台端',
  modePlatformHint: '作为中心节点，管理下级网关',
  modeGateway: '下级网关',
  modeGatewayHint: '作为边缘节点，向上级平台上报数据',
  mqttTitle: 'MQTT 连接配置',
  mqttUrl: 'MQTT Broker 地址',
  mqttUrlHint: '平台端和网关端都需要连接到此独立部署的 MQTT Broker',
  username: '认证用户名',
  password: '认证密码',
  enableTls: '启用 TLS 加密',
  enableTlsHint: '为级联 MQTT 连接开启 TLS 加密',
  skipVerify: '跳过证书校验',
  skipVerifyHint: '自签名证书环境下允许跳过证书校验',
  gatewayTitle: '网关专属配置',
  gatewaySn: '网关序列号 (SN)',
  gatewaySnHint: '用于唯一标识本网关设备，平台将通过此 SN 识别网关',
  gatewayName: '网关名称',
  gatewayNameHint: '网关名称，注册时上报',
  cancel: '取消',
  saving: '保存中…',
  save: '保存并立即生效',
  savedToast: '保存成功，级联插件已应用新配置',
  saveFailed: '保存失败',
  saveFailedWithReason: (message) => `保存失败：${message}`
} : {
  modeTitle: 'Operating mode',
  modePlatform: 'Platform',
  modePlatformHint: 'Acts as the central node and manages downstream gateways',
  modeGateway: 'Gateway',
  modeGatewayHint: 'Acts as the edge node and reports data to the upstream platform',
  mqttTitle: 'MQTT configuration',
  mqttUrl: 'MQTT broker address',
  mqttUrlHint: 'Both the platform and the gateway connect to this standalone MQTT broker',
  username: 'Username',
  password: 'Password',
  enableTls: 'Enable TLS encryption',
  enableTlsHint: 'Encrypt the cascade MQTT connection with TLS',
  skipVerify: 'Skip certificate verification',
  skipVerifyHint: 'Allow skipping verification when a self-signed certificate is used',
  gatewayTitle: 'Gateway-specific configuration',
  gatewaySn: 'Gateway serial number (SN)',
  gatewaySnHint: 'Uniquely identifies this gateway; the platform recognises it by this SN',
  gatewayName: 'Gateway name',
  gatewayNameHint: 'Reported to the platform when the gateway registers',
  cancel: 'Cancel',
  saving: 'Saving…',
  save: 'Save and apply now',
  savedToast: 'Saved. The cascade plugin applied the new configuration.',
  saveFailed: 'Save failed',
  saveFailedWithReason: (message) => `Save failed: ${message}`
}));

const loading = ref(true);
const saving = ref(false);

const config = ref({
  mode: 'platform',
  mqtt_url: 'tcp://127.0.0.1:1883',
  enable_tls: false,
  insecure_skip_verify: false,
  username: '',
  password: '',
  gateway_sn: '',
  gateway_name: ''
});

const configEndpoint = computed(() => props.apiBase || `/api/plugins/${props.pluginName || 'cascade'}`);
const saveEndpoint = computed(() => `${configEndpoint.value}/config`);

const loadConfig = async () => {
  try {
    const res = await axios.get(configEndpoint.value);
    const schema = res.data?.data?.schema || res.data?.data;
    if (res.data.code === 0 && schema && schema.fields) {
      const fields = schema.fields;
      const getVal = (name, defaultVal) => {
        const f = fields.find(x => x.name === name);
        return f && f.value !== undefined ? f.value : defaultVal;
      };

      config.value.mode = getVal('mode', 'platform');
      config.value.mqtt_url = getVal('mqtt_url', 'tcp://127.0.0.1:1883');
      config.value.enable_tls = Boolean(getVal('enable_tls', false));
      config.value.insecure_skip_verify = Boolean(getVal('insecure_skip_verify', false));
      config.value.username = getVal('username', '');
      config.value.password = getVal('password', '');
      config.value.gateway_sn = getVal('gateway_sn', '');
      config.value.gateway_name = getVal('gateway_name', '');
    }
    
  } catch (e) {
    console.error('Failed to load cascade config', e);
  } finally {
    loading.value = false;
  }
};

onMounted(loadConfig);

async function save() {
  saving.value = true;
  try {
    const payload = {
      mode: config.value.mode,
      mqtt_url: config.value.mqtt_url,
      enable_tls: config.value.enable_tls,
      insecure_skip_verify: config.value.insecure_skip_verify,
      username: config.value.username,
      password: config.value.password,
      gateway_sn: config.value.gateway_sn,
      gateway_name: config.value.gateway_name
    };
    
    if (props.remoteContext?.baseVersion !== undefined) {
      payload.base_version = props.remoteContext.baseVersion;
    }

    const res = await axios.post(saveEndpoint.value, payload);
    if (res.data.code === 0) {
      showToast('success', C.value.savedToast);
      emit('saved', res.data.data);
      window.dispatchEvent(new CustomEvent('cascade-config-updated', { detail: res.data.data }));
      if (!props.apiBase) {
        router.back();
      }
    } else {
      showToast('danger', res.data.message || C.value.saveFailed);
    }
  } catch (e) {
    showToast('danger', C.value.saveFailedWithReason(e.message));
  } finally {
    saving.value = false;
  }
}

function cancel() {
  emit('cancel');
  if (!props.apiBase) {
    router.back();
  }
}
</script>

<style scoped>
.cascade-config-panel {
  max-width: 800px;
  margin: 0 auto;
}
</style>
