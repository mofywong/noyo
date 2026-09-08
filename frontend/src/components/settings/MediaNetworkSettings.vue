<template>
  <form class="card table-glass-card media-network mb-4" @submit.prevent="save" aria-labelledby="media-network-title" autocomplete="off">
    <div class="card-body">
      <h5 id="media-network-title">{{ text('平台 WebRTC 网络', 'Platform WebRTC network') }}</h5>
      <p class="text-muted">{{ text('这里配置平台网页点播使用的 STUN/TURN。平台会在每次播放会话中把配置下发给对应网关。', 'Configure STUN/TURN for platform web playback. The settings are sent to the target gateway with each playback session.') }}</p>
      <p v-if="managed" role="status" class="text-info">{{ text('当前配置由部署环境管理，请在部署环境中修改。', 'This configuration is managed by deployment environment variables.') }}</p>
      <p v-if="error" class="text-danger" role="alert">{{ error }}</p>
      <p v-if="success" class="text-success" role="status">{{ text('平台媒体网络配置已保存。', 'Platform media network configuration saved.') }}</p>
      <p v-if="loading" role="status">{{ text('正在加载…', 'Loading…') }}</p>
      <fieldset :disabled="loading || saving || managed || !loaded">
        <div class="row g-3">
          <div v-for="field in urlFields" :key="field.key" class="col-12">
            <label :for="`media-${field.key}`" class="form-label">{{ field.label }}</label>
            <textarea :id="`media-${field.key}`" v-model="form[field.key]" class="form-control" rows="2" :placeholder="field.placeholder" />
          </div>
          <p class="text-muted mb-0">{{ text('多个地址使用逗号分隔。留空表示不使用该服务。', 'Separate multiple URLs with commas. Leave blank to disable the service.') }}</p>
          <div class="col-md-6">
            <label for="media-auth" class="form-label">{{ text('TURN 认证方式', 'TURN authentication') }}</label>
            <select id="media-auth" v-model="form.auth_mode" class="form-select"><option value="rest">{{ text('临时凭据（共享密钥）', 'Temporary credentials (shared secret)') }}</option><option value="static">{{ text('固定用户名和密码', 'Static username and password') }}</option></select>
          </div>
          <div v-if="form.auth_mode === 'rest'" class="col-md-6"><label for="media-ttl" class="form-label">{{ text('临时凭据有效期（秒）', 'Credential lifetime (seconds)') }}</label><input id="media-ttl" v-model.number="form.credential_ttl_seconds" type="number" min="60" max="86400" step="1" class="form-control" required></div>
          <div v-if="form.auth_mode === 'static'" class="col-md-6"><label for="media-username" class="form-label">{{ text('TURN 用户名', 'TURN username') }}</label><input id="media-username" v-model="form.turn_username" class="form-control" autocomplete="off"></div>
          <div class="col-12">
            <label for="media-credential" class="form-label">{{ form.auth_mode === 'rest' ? text('TURN 共享密钥', 'TURN shared secret') : text('TURN 密码', 'TURN password') }}</label>
            <input id="media-credential" v-model="form[credentialKey]" type="password" class="form-control" autocomplete="new-password" :disabled="form[clearKey]" aria-describedby="media-credential-hint">
            <div id="media-credential-hint" class="form-text">{{ configured ? text('已配置。留空表示保留现有凭据，填写新值可替换。', 'Configured. Leave blank to retain the existing credential, or enter a replacement.') : text('尚未配置。', 'Not configured.') }}</div>
            <button v-if="configured" type="button" class="btn btn-outline-danger mt-2" @click="clearCredential">{{ form[clearKey] ? text('撤销清除', 'Undo clear') : text('清除已保存凭据', 'Clear saved credential') }}</button>
            <p v-if="form[clearKey]" class="text-warning mt-2" role="status">{{ text('保存后将清除该凭据。', 'The credential will be cleared when you save.') }}</p>
          </div>
        </div>
        <div class="mt-4 text-end"><button type="submit" class="btn btn-primary">{{ saving ? text('保存中…', 'Saving…') : text('保存平台网络配置', 'Save platform network') }}</button></div>
      </fieldset>
    </div>
  </form>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import axios from 'axios'
import { useConfirm } from '../../composables/useConfirm'
const { locale } = useI18n(); const text = (zh, en) => String(locale.value).startsWith('zh') ? zh : en
const { confirmDialog } = useConfirm()
const loading = ref(true); const saving = ref(false); const loaded = ref(false); const error = ref(''); const success = ref(false)
const form = reactive({ stun_urls: '', turn_urls: '', auth_mode: 'rest', turn_username: '', credential_ttl_seconds: 3600, source: '', turn_secret: '', turn_password: '', clear_turn_secret: false, clear_turn_password: false })
const managed = computed(() => form.source === 'environment')
const credentialKey = computed(() => form.auth_mode === 'rest' ? 'turn_secret' : 'turn_password')
const clearKey = computed(() => `clear_${credentialKey.value}`)
const configured = computed(() => !!form[`${credentialKey.value}_configured`])
const urlFields = computed(() => [{ key: 'stun_urls', label: text('STUN 地址', 'STUN URLs'), placeholder: 'stun:stun.example.com:3478' }, { key: 'turn_urls', label: text('TURN 地址', 'TURN URLs'), placeholder: 'turn:turn.example.com:3478' }])
const accept = data => { Object.assign(form, data, { turn_secret: '', turn_password: '', clear_turn_secret: false, clear_turn_password: false }); loaded.value = true }
const clearCredential = async () => {
  if (form[clearKey.value]) { form[clearKey.value] = false; return }
  const key = clearKey.value
  if (await confirmDialog({ title: text('清除媒体凭据', 'Clear media credential'), message: text('清除后，使用该凭据的播放可能无法通过 TURN 中继。确定继续吗？', 'Future playback using this credential may lose TURN relay access. Clear it?'), confirmText: text('清除', 'Clear'), cancelText: text('取消', 'Cancel') })) { form[key] = true; form[key.replace('clear_', '')] = '' }
}
const save = async () => {
  if (managed.value || saving.value || !loaded.value) return
  saving.value = true; error.value = ''; success.value = false
  try {
    const { stun_urls, turn_urls, auth_mode, turn_username, credential_ttl_seconds, turn_secret, turn_password, clear_turn_secret, clear_turn_password } = form
    const res = await axios.put('/api/system/media-network', { stun_urls, turn_urls, auth_mode, turn_username, credential_ttl_seconds, turn_secret, turn_password, clear_turn_secret, clear_turn_password })
    if (res.data?.code !== 0 || !res.data.data) throw new Error(res.data?.message || text('保存失败', 'Save failed'))
    accept(res.data.data); success.value = true
  } catch (err) { error.value = err.response?.data?.message || err.message } finally { saving.value = false }
}
onMounted(async () => { try { const res = await axios.get('/api/system/media-network'); if (res.data?.code !== 0 || !res.data.data) throw new Error(res.data?.message || text('加载失败', 'Load failed')); accept(res.data.data) } catch (err) { error.value = err.response?.data?.message || err.message } finally { loading.value = false } })
</script>

<style scoped>
.media-network { max-width: 960px; color: var(--text-primary); }
.card-body { padding: 24px; }
fieldset { min-width: 0; }
</style>
