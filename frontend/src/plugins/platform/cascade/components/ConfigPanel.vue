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
            <i class="bi bi-hdd-network me-2 text-primary"></i>运行模式 (Operating Mode)
          </div>
          <div class="card-body">
            <div class="row">
              <div class="col-md-6">
                <div class="form-check form-check-inline border rounded p-3 w-100 text-center"
                     :class="{'border-primary bg-primary-subtle': config.mode === 'platform'}"
                     @click="config.mode = 'platform'" style="cursor: pointer;">
                  <input class="form-check-input float-none ms-0 mb-2 d-block mx-auto" type="radio" 
                         name="mode" value="platform" v-model="config.mode">
                  <label class="form-check-label fw-bold d-block">平台端 (Platform)</label>
                  <small class="text-muted d-block mt-1">作为中心节点，管理下级网关</small>
                </div>
              </div>
              <div class="col-md-6">
                <div class="form-check form-check-inline border rounded p-3 w-100 text-center"
                     :class="{'border-primary bg-primary-subtle': config.mode === 'gateway'}"
                     @click="config.mode = 'gateway'" style="cursor: pointer;">
                  <input class="form-check-input float-none ms-0 mb-2 d-block mx-auto" type="radio" 
                         name="mode" value="gateway" v-model="config.mode">
                  <label class="form-check-label fw-bold d-block">下级网关 (Gateway)</label>
                  <small class="text-muted d-block mt-1">作为边缘节点，向上级平台上报数据</small>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- MQTT Config -->
        <div class="card mb-4 border-0 shadow-sm">
          <div class="card-header bg-light fw-bold">
            <i class="bi bi-server me-2 text-primary"></i>MQTT 连接配置 (MQTT Config)
          </div>
          <div class="card-body">
            <div class="mb-3">
              <label class="form-label fw-bold small">MQTT Broker 地址 <span class="text-danger">*</span></label>
              <input type="text" class="form-control" v-model="config.mqtt_url" placeholder="tcp://127.0.0.1:1883" required>
              <div class="form-text">平台端和网关端都需要连接到此独立部署的 MQTT Broker</div>
            </div>
            <div class="row">
              <div class="col-md-6 mb-3">
                <label class="form-label fw-bold small">认证用户名</label>
                <input type="text" class="form-control" v-model="config.username" autocomplete="new-password">
              </div>
              <div class="col-md-6 mb-3">
                <label class="form-label fw-bold small">认证密码</label>
                <input type="password" class="form-control" v-model="config.password" autocomplete="new-password">
              </div>
            </div>
          </div>
        </div>

        <!-- Gateway Config -->
        <div v-if="config.mode === 'gateway'" class="card mb-4 border-0 shadow-sm">
          <div class="card-header bg-light fw-bold">
            <i class="bi bi-router me-2 text-success"></i>网关专属配置 (Gateway Config)
          </div>
          <div class="card-body">
            <div class="mb-3">
              <label class="form-label fw-bold small">网关序列号 (SN) <span class="text-danger">*</span></label>
              <input type="text" class="form-control" v-model="config.gateway_sn" :required="config.mode === 'gateway'">
              <div class="form-text">用于唯一标识本网关设备，平台将通过此 SN 识别网关</div>
            </div>
            <div class="mb-3">
              <label class="form-label fw-bold small">网关名称 (Name)</label>
              <input type="text" class="form-control" v-model="config.gateway_name">
              <div class="form-text">网关名称，注册时上报</div>
            </div>
          </div>
        </div>

        <!-- Actions -->
        <div class="d-flex justify-content-end pt-3 border-top gap-2">
          <button type="button" class="btn btn-light" @click="$router.back()">取消</button>
          <button type="submit" class="btn btn-primary px-4" :disabled="saving">
            <span v-if="saving" class="spinner-border spinner-border-sm me-2"></span>
            <i v-else class="bi bi-save me-2"></i>
            {{ saving ? '保存中...' : '保存并立即生效' }}
          </button>
        </div>
      </form>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue';
import axios from 'axios';
import { useToast } from '@/composables/useToast';
import { useRouter } from 'vue-router';

const props = defineProps({ pluginName: String });
const { showToast } = useToast();
const router = useRouter();

const loading = ref(true);
const saving = ref(false);

const config = ref({
  mode: 'platform',
  mqtt_url: 'tcp://127.0.0.1:1883',
  username: '',
  password: '',
  gateway_sn: '',
  gateway_name: ''
});

onMounted(async () => {
  try {
    const res = await axios.get(`/api/plugins/${props.pluginName || 'cascade'}`);
    if (res.data.code === 0 && res.data.data && res.data.data.fields) {
      const fields = res.data.data.fields;
      const getVal = (name, defaultVal) => {
        const f = fields.find(x => x.name === name);
        return f && f.value !== undefined ? f.value : defaultVal;
      };

      config.value.mode = getVal('mode', 'platform');
      config.value.mqtt_url = getVal('mqtt_url', 'tcp://127.0.0.1:1883');
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
});

onUnmounted(() => {
});

async function save() {
  saving.value = true;
  try {
    const payload = {
      mode: config.value.mode,
      mqtt_url: config.value.mqtt_url,
      username: config.value.username,
      password: config.value.password,
      gateway_sn: config.value.gateway_sn,
      gateway_name: config.value.gateway_name
    };
    
    const res = await axios.post(`/api/plugins/${props.pluginName || 'cascade'}/config`, payload);
    if (res.data.code === 0) {
      showToast('success', '保存成功，级联插件已应用新配置');
      router.back();
    } else {
      showToast('danger', res.data.message || '保存失败');
    }
  } catch (e) {
    showToast('danger', '保存失败：' + e.message);
  } finally {
    saving.value = false;
  }
}
</script>

<style scoped>
.cascade-config-panel {
  max-width: 800px;
  margin: 0 auto;
}
</style>
