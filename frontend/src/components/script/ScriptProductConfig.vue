<template>
  <div class="card border-0 shadow-sm">
    <div class="card-header bg-white py-3">
      <div class="d-flex align-items-center justify-content-between">
        <h5 class="mb-0 text-primary">
          <i class="bi bi-code-square me-2"></i>{{ $t('script_driver_config') }}
        </h5>
        <ul class="nav nav-pills card-header-pills">
          <li class="nav-item">
            <a class="nav-link" :class="{ active: activeTab === 'params' }" href="#" @click.prevent="activeTab = 'params'">{{ $t('script_device_params') }}</a>
          </li>
          <li class="nav-item">
            <a class="nav-link" :class="{ active: activeTab === 'script' }" href="#" @click.prevent="activeTab = 'script'">{{ $t('script_tab_script') }}</a>
          </li>
        </ul>
      </div>
    </div>

    <!-- Params Mode Tab -->
    <div v-if="activeTab === 'params'" class="p-3">
       <div v-if="deviceParams.length === 0" class="text-center text-muted py-5">
         <p>{{ $t('script_no_params') }}</p>
         <button class="btn btn-primary btn-sm" @click="addParam">
           <i class="bi bi-plus"></i> {{ $t('script_add_param') }}
         </button>
       </div>
       <div v-else>
         <table class="table table-borderless align-middle">
           <thead>
             <tr>
               <th style="width: 25%">{{ $t('script_param_key') }}</th>
               <th style="width: 25%">{{ $t('script_param_name') }}</th>
               <th style="width: 20%">{{ $t('script_param_type') }}</th>
               <th style="width: 15%">{{ $t('script_param_required') }}</th>
               <th style="width: 15%"></th>
             </tr>
           </thead>
           <tbody>
             <tr v-for="(param, index) in deviceParams" :key="index">
               <td>
                 <input type="text" class="form-control" v-model="param.key" @input="updateParams" placeholder="e.g. ip">
               </td>
               <td>
                 <input type="text" class="form-control" v-model="param.name" @input="updateParams" placeholder="e.g. IP Address">
               </td>
               <td>
                 <select class="form-select" v-model="param.type" @change="updateParams">
                   <option value="string">String</option>
                   <option value="number">Number</option>
                   <option value="boolean">Boolean</option>
                 </select>
               </td>
               <td>
                 <div class="form-check form-switch">
                   <input class="form-check-input" type="checkbox" v-model="param.required" @change="updateParams">
                 </div>
               </td>
               <td>
                 <button class="btn btn-outline-danger btn-sm" @click="removeParam(index)">
                   <i class="bi bi-trash"></i>
                 </button>
               </td>
             </tr>
           </tbody>
         </table>
         <button class="btn btn-outline-primary btn-sm mt-2" @click="addParam">
           <i class="bi bi-plus"></i> {{ $t('script_add_param') }}
         </button>
       </div>
    </div>

    <!-- Script Editor Tab -->
    <div v-else-if="activeTab === 'script'" class="p-3" :class="{ 'editor-fullscreen': isFullscreen }">
      <!-- Protocol Selection / Generator (Hidden in Fullscreen) -->
      <div class="mb-3 p-3 bg-light rounded border" v-if="!isFullscreen">
        <div class="mb-3">
            <label class="form-label fw-bold mb-2">{{ $t('script_select_protocols') }}</label>
            <div class="d-flex gap-3 flex-wrap align-items-center bg-white p-2 rounded border">
                <!-- HTTP -->
                <div class="form-check">
                    <input class="form-check-input" type="checkbox" value="http_client" v-model="selectedProtocols" id="proto_http_client">
                    <label class="form-check-label" for="proto_http_client">{{ $t('script_proto_http_client') }}</label>
                </div>
                <div class="form-check">
                    <input class="form-check-input" type="checkbox" value="http_server" v-model="selectedProtocols" id="proto_http_server">
                    <label class="form-check-label" for="proto_http_server">{{ $t('script_proto_http_server') }}</label>
                </div>
                
                <!-- TCP -->
                <div class="form-check">
                    <input class="form-check-input" type="checkbox" value="tcp_client" v-model="selectedProtocols" id="proto_tcp_client">
                    <label class="form-check-label" for="proto_tcp_client">{{ $t('script_proto_tcp_client') }}</label>
                </div>
                <div class="form-check">
                    <input class="form-check-input" type="checkbox" value="tcp_server" v-model="selectedProtocols" id="proto_tcp_server">
                    <label class="form-check-label" for="proto_tcp_server">{{ $t('script_proto_tcp_server') }}</label>
                </div>

                <!-- UDP -->
                <div class="form-check">
                    <input class="form-check-input" type="checkbox" value="udp_client" v-model="selectedProtocols" id="proto_udp_client">
                    <label class="form-check-label" for="proto_udp_client">{{ $t('script_proto_udp_client') }}</label>
                </div>
                <div class="form-check">
                    <input class="form-check-input" type="checkbox" value="udp_server" v-model="selectedProtocols" id="proto_udp_server">
                    <label class="form-check-label" for="proto_udp_server">{{ $t('script_proto_udp_server') }}</label>
                </div>

                <!-- MQTT -->
                <div class="form-check">
                    <input class="form-check-input" type="checkbox" value="mqtt_client" v-model="selectedProtocols" id="proto_mqtt_client">
                    <label class="form-check-label" for="proto_mqtt_client">{{ $t('script_proto_mqtt_client') }}</label>
                </div>
            </div>
        </div>
        <div class="mb-3">
            <label class="form-label fw-bold mb-2">{{ $t('script_select_modules') }}</label>
            <div class="d-flex gap-3 flex-wrap align-items-center mb-2 bg-white p-2 rounded border">
                <!-- JSON -->
                <div class="form-check d-flex align-items-center">
                    <input class="form-check-input me-2" type="checkbox" value="json" v-model="selectedModules" id="mod_json">
                    <label class="form-check-label me-1" for="mod_json">{{ $t('script_mod_json') }}</label>
                    <i class="bi bi-question-circle text-muted" style="font-size: 0.9rem; cursor: help;" :title="$t('script_mod_json_desc')"></i>
                </div>
                <!-- Struct -->
                <div class="form-check d-flex align-items-center">
                    <input class="form-check-input me-2" type="checkbox" value="struct" v-model="selectedModules" id="mod_struct">
                    <label class="form-check-label me-1" for="mod_struct">{{ $t('script_mod_struct') }}</label>
                    <i class="bi bi-question-circle text-muted" style="font-size: 0.9rem; cursor: help;" :title="$t('script_mod_struct_desc')"></i>
                </div>
                <!-- Crypto -->
                <div class="form-check d-flex align-items-center">
                    <input class="form-check-input me-2" type="checkbox" value="crypto" v-model="selectedModules" id="mod_crypto">
                    <label class="form-check-label me-1" for="mod_crypto">{{ $t('script_mod_crypto') }}</label>
                    <i class="bi bi-question-circle text-muted" style="font-size: 0.9rem; cursor: help;" :title="$t('script_mod_crypto_desc')"></i>
                </div>
            </div>
        </div>
        <button class="btn btn-primary w-100" @click="generateScript" :disabled="selectedProtocols.length === 0 && selectedModules.length === 0">
            <i class="bi bi-magic me-1"></i> {{ $t('script_generate_template') }}
        </button>
        <div class="form-text text-muted mt-1 text-center">
            <i class="bi bi-exclamation-triangle me-1"></i> {{ $t('script_generate_warning') }}
        </div>
      </div>

      <!-- Editor Toolbar -->
      <div class="d-flex justify-content-between mb-2 align-items-center">
         <span class="text-muted small" v-if="!isFullscreen"><i class="bi bi-info-circle me-1"></i>{{ $t('script_editor_hint') }}</span>
         <div class="ms-auto">
             <button class="btn btn-sm btn-outline-secondary me-2" @click="copyScript">
                <i class="bi bi-clipboard me-1"></i> {{ $t('script_copy') }}
             </button>
             <button class="btn btn-sm btn-outline-secondary me-2" @click="downloadScript">
                <i class="bi bi-download me-1"></i> {{ $t('script_download') }}
             </button>
             <button class="btn btn-sm btn-outline-secondary me-2" @click="toggleFullscreen">
                <i class="bi" :class="isFullscreen ? 'bi-fullscreen-exit' : 'bi-fullscreen'"></i> {{ isFullscreen ? $t('script_exit_fullscreen') : $t('script_fullscreen') }}
             </button>
             <button class="btn btn-sm btn-outline-warning" @click="startDebug">
                <i class="bi bi-bug me-1"></i> {{ $t('script_debug') }}
             </button>
         </div>
      </div>

      <!-- Editor -->
      <div class="script-editor-container" :style="{ height: isFullscreen ? 'calc(100vh - 80px)' : '600px' }">
        <VueMonacoEditor
            v-model:value="scriptContent"
            language="lua"
            theme="vs-dark"
            :options="{
                automaticLayout: true,
                formatOnType: true,
                formatOnPaste: true,
                minimap: { enabled: false },
                fontSize: 14
            }"
            @mount="handleMount"
            @change="onEditorChange"
        />
      </div>
    </div>

    <!-- Device Selection Modal -->
    <div v-if="showDeviceSelect" class="modal fade show d-block" style="background: rgba(0,0,0,0.5); z-index: 1070;">
      <div class="modal-dialog">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">{{ $t('script_select_device_debug') }}</h5>
            <button type="button" class="btn-close" @click="showDeviceSelect = false"></button>
          </div>
          <div class="modal-body">
            <div v-if="devices.length === 0" class="text-center text-muted">
              {{ $t('script_no_devices') }}
            </div>
            <div v-else class="list-group">
              <button v-for="d in devices" :key="d.code" 
                  class="list-group-item list-group-item-action"
                  @click="selectDevice(d)">
                {{ d.name }} <span class="text-muted small">({{ d.code }})</span>
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Debug Modal -->
    <ScriptDebugConsole v-if="showDebug" :device-code="selectedDeviceCode" @close="showDebug = false" />
  </div>
</template>

<script setup>
import { ref, onMounted, watch, nextTick } from 'vue';
import axios from 'axios';
import { useI18n } from 'vue-i18n';
import ScriptDebugConsole from './ScriptDebugConsole.vue';
import { VueMonacoEditor } from '@guolao/vue-monaco-editor';

const props = defineProps({
  modelValue: { type: Object, default: () => ({}) },
  productCode: { type: String, default: '' }
});
const emit = defineEmits(['update:modelValue']);
const { t } = useI18n();

const activeTab = ref('params');
const selectedProtocols = ref([]);
const selectedModules = ref([]);
const scriptContent = ref('');
const showDebug = ref(false);
const showDeviceSelect = ref(false);
const devices = ref([]);
const selectedDeviceCode = ref('');
const deviceParams = ref([]);
const isFullscreen = ref(false);
let monacoInstance = null;

const inferProtocolsAndModules = (script) => {
    if (!script) return;
    
    // Strip comments to avoid false positives
    // 1. Block comments --[[ ... ]]
    // 2. Single line comments -- ...
    const cleanScript = script.replace(/--\[\[[\s\S]*?\]\]/g, '').replace(/--.*$/gm, '');

    const protocols = new Set();
    const modules = new Set();

    // Protocols - Usage Detection
    if (cleanScript.includes('http.get') || cleanScript.includes('http.post') || cleanScript.includes('http.request')) protocols.add('http_client');
    if (/clients\.http\s*=/.test(cleanScript) || /clients\[['"]http['"]\]\s*=/.test(cleanScript)) protocols.add('http_client');
    if (cleanScript.includes('listeners.http') || /servers\.http\s*=/.test(cleanScript) || /servers\[['"]http['"]\]\s*=/.test(cleanScript)) protocols.add('http_server');
    if (cleanScript.includes('net.tcp_request') || cleanScript.includes('net.dial_tcp')) protocols.add('tcp_client');
    if (/clients\.tcp\s*=/.test(cleanScript) || /clients\[['"]tcp['"]\]\s*=/.test(cleanScript)) protocols.add('tcp_client');
    if (cleanScript.includes('listeners.tcp') || /servers\.tcp\s*=/.test(cleanScript) || /servers\[['"]tcp['"]\]\s*=/.test(cleanScript)) protocols.add('tcp_server');
    if (cleanScript.includes('net.udp_request') || cleanScript.includes('net.dial_udp')) protocols.add('udp_client');
    if (cleanScript.includes('listeners.udp') || /servers\.udp\s*=/.test(cleanScript) || /servers\[['"]udp['"]\]\s*=/.test(cleanScript)) protocols.add('udp_server');
    if (cleanScript.includes('listeners.mqtt') || cleanScript.includes('mqtt.client') || /clients\.mqtt\s*=/.test(cleanScript) || /clients\[['"]mqtt['"]\]\s*=/.test(cleanScript)) protocols.add('mqtt_client');

    // Modules
    if (cleanScript.includes('require("json")') || cleanScript.includes("require('json')")) modules.add('json');
    if (cleanScript.includes('require("struct")') || cleanScript.includes("require('struct')")) modules.add('struct');
    if (cleanScript.includes('require("crypto")') || cleanScript.includes("require('crypto')")) modules.add('crypto');

    selectedProtocols.value = Array.from(protocols);
    selectedModules.value = Array.from(modules);
};

const handleMount = (editor, monaco) => {
    monacoInstance = monaco;
    // Register completion provider
    monaco.languages.registerCompletionItemProvider('lua', {
        provideCompletionItems: (model, position) => {
            const word = model.getWordUntilPosition(position);
            const range = {
                startLineNumber: position.lineNumber,
                endLineNumber: position.lineNumber,
                startColumn: word.startColumn,
                endColumn: word.endColumn
            };

            const suggestions = [
                // Global Objects
                { label: 'sys', kind: monaco.languages.CompletionItemKind.Module, insertText: 'sys', detail: 'System Module' },
                { label: 'http', kind: monaco.languages.CompletionItemKind.Module, insertText: 'http', detail: 'HTTP Module' },
                { label: 'net', kind: monaco.languages.CompletionItemKind.Module, insertText: 'net', detail: 'Network Module' },
                { label: 'mqtt', kind: monaco.languages.CompletionItemKind.Module, insertText: 'mqtt', detail: 'MQTT Module' },
                { label: 'json', kind: monaco.languages.CompletionItemKind.Module, insertText: 'json', detail: 'JSON Module' },
                { label: 'struct', kind: monaco.languages.CompletionItemKind.Module, insertText: 'struct', detail: 'Struct Module' },
                { label: 'crypto', kind: monaco.languages.CompletionItemKind.Module, insertText: 'crypto', detail: 'Crypto Module' },
                { label: 'ctx', kind: monaco.languages.CompletionItemKind.Variable, insertText: 'ctx', detail: 'Context Object' },
                
                // Methods
                { label: 'ctx:config', kind: monaco.languages.CompletionItemKind.Method, insertText: 'ctx:config("${1:key}")', insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet, detail: 'Get Config' },
                { label: 'ctx:log', kind: monaco.languages.CompletionItemKind.Method, insertText: 'ctx:log("${1:info}", "${2:msg}")', insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet, detail: 'Log Message' },
                { label: 'ctx:get', kind: monaco.languages.CompletionItemKind.Method, insertText: 'ctx:get("${1:key}")', insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet, detail: 'Get Context Data' },
                { label: 'ctx:set', kind: monaco.languages.CompletionItemKind.Method, insertText: 'ctx:set("${1:key}", ${2:val})', insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet, detail: 'Set Context Data' },
                { label: 'ctx:report_status', kind: monaco.languages.CompletionItemKind.Method, insertText: 'ctx:report_status(${1:status_or_table})', insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet, detail: 'Report Device/Sub-Device Status' },
                { label: 'ctx:update_device_mapping', kind: monaco.languages.CompletionItemKind.Method, insertText: 'ctx:update_device_mapping("${1:device_code}", "${2:external_id}")', insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet, detail: 'Update Device Code to External ID Mapping' },
                { label: 'ctx:get_sub_devices', kind: monaco.languages.CompletionItemKind.Method, insertText: 'ctx:get_sub_devices()', detail: 'Get All Sub-Devices with Config' },
                
                { label: 'sys.now()', kind: monaco.languages.CompletionItemKind.Function, insertText: 'sys.now()', detail: 'Get Current Timestamp' },
                { label: 'sys.sleep', kind: monaco.languages.CompletionItemKind.Function, insertText: 'sys.sleep(${1:ms})', insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet, detail: 'Sleep (ms)' },
                
                { label: 'json.encode', kind: monaco.languages.CompletionItemKind.Function, insertText: 'json.encode(${1:obj})', insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet, detail: 'Encode JSON' },
                { label: 'json.decode', kind: monaco.languages.CompletionItemKind.Function, insertText: 'json.decode(${1:str})', insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet, detail: 'Decode JSON' },
            ];

            // Add device params
            if (deviceParams.value) {
                deviceParams.value.forEach(p => {
                    suggestions.push({
                        label: 'config.' + p.key,
                        kind: monaco.languages.CompletionItemKind.Property,
                        insertText: 'config.' + p.key,
                        detail: p.name + ' (' + p.type + ')',
                        range: range
                    });
                });
            }

            return { suggestions: suggestions };
        }
    });
};

// Sync prop to local state
watch(() => props.modelValue, (val) => {
  if (val) {
    if (val.script) {
      scriptContent.value = val.script;
      inferProtocolsAndModules(val.script);
    }
    if (val.device_params && Array.isArray(val.device_params)) {
        deviceParams.value = JSON.parse(JSON.stringify(val.device_params));
    } else {
        deviceParams.value = [];
    }
  }
}, { immediate: true });

const onEditorChange = (val) => {
    scriptContent.value = val;
    inferProtocolsAndModules(val);
    updateParams(val);
};

const updateParams = (val) => {
    // Check if val is the editor content (string) or an event object
    const content = (typeof val === 'string') ? val : scriptContent.value;
    
    // console.log('ScriptProductConfig: Emitting update', content.substring(0, 20) + '...');
    
    emit('update:modelValue', { 
        ...props.modelValue, 
        script: content,
        device_params: deviceParams.value
    });
};

const addParam = () => {
    deviceParams.value.push({ key: '', name: '', type: 'string', required: false });
    updateParams();
};

const removeParam = (index) => {
    deviceParams.value.splice(index, 1);
    updateParams();
};

const toggleFullscreen = () => {
    isFullscreen.value = !isFullscreen.value;
};

const generateScript = () => {
  if (selectedProtocols.value.length === 0 && selectedModules.value.length === 0 && !confirm(t('script_no_protocols'))) {
      return;
  }
  if (scriptContent.value && scriptContent.value.trim() !== '' && !confirm(t('script_overwrite_confirm'))) {
      return;
  }

  try {
    let imports = [];
    // Standard modules
    imports.push('local sys = require("sys")');
    imports.push('local json = require("json")');

    // Optional modules
    if (selectedModules.value.includes('struct')) imports.push('local struct = require("struct")');
    if (selectedModules.value.includes('http') || selectedProtocols.value.includes('http_client')) imports.push('local http = require("http")');
    if (selectedModules.value.includes('crypto')) imports.push('local crypto = require("crypto")');
    if (selectedModules.value.includes('net') || selectedProtocols.value.includes('tcp_client') || selectedProtocols.value.includes('udp_client')) imports.push('local net = require("net")');
    if (selectedProtocols.value.includes('mqtt_client')) imports.push('local mqtt = require("mqtt")');
    
    // Encoding modules (always available if needed, but explicit require helps clarity)
    imports.push('-- local hex = require("hex")');
    imports.push('-- local base64 = require("base64")');

    let script = imports.join('\n') + '\n\n';

    // 1. Lifecycle: Initialization
    script += `----------------------------------------------------------------
-- 1. Lifecycle: Initialization / 生命周期：初始化
----------------------------------------------------------------
-- Called when the plugin is initialized.
-- 插件初始化时调用。
-- Used to validate configuration and register protocol listeners.
-- 用于校验配置并注册协议监听器。
-- @param ctx: Context object (methods: log, set, get, config, ...) / 上下文对象
-- @return success (bool): true if initialization is successful / 初始化是否成功
-- @return listeners (table): table of listeners to start / 需启动的监听器列表
function on_init(ctx)
    -- [Example] Validate configuration / [示例] 校验配置
    -- local host = ctx:config("host")
    -- if not host then 
    --     return false, "Missing host configuration" 
    -- end

    ctx:log("info", "Plugin initializing...")

    -- [Example] Return server and client configurations / [示例] 返回服务端和客户端配置
    local servers = {}
    local clients = {}
`;

    if (selectedProtocols.value.includes('http_server')) {
        script += `
    -- HTTP Server Listener Configuration / HTTP 服务端监听配置
    servers.http = { 
        port = 8989,               -- [Required] Listening port / [必填] 监听端口
        tls = false,               -- [Optional] Enable TLS / [可选] 是否启用 TLS
        cert_file = "cert.pem",    -- [Optional] Cert file path (when tls=true) / [可选] 证书文件路径
        key_file = "key.pem",      -- [Optional] Key file path (when tls=true) / [可选] 私钥文件路径
        max_connections = 1024,    -- [Optional] Max concurrent connections / [可选] 最大并发连接数
        read_timeout = 5000,       -- [Optional] Read timeout (ms) / [可选] 读取超时 (毫秒)
        write_timeout = 5000,      -- [Optional] Write timeout (ms) / [可选] 写入超时 (毫秒)
        idle_timeout = 30000,      -- [Optional] Idle timeout (ms) / [可选] 空闲连接超时 (毫秒)
        max_header_bytes = 1048576 -- [Optional] Max header size (bytes) / [可选] 最大请求头大小
    }
`;
    }
    
    if (selectedProtocols.value.includes('tcp_server')) {
        script += `
    -- TCP Server Listener Configuration / TCP 服务端监听配置
    servers.tcp = { 
        port = 8888,               -- [Required] Listening port / [必填] 监听端口
        framer = "lines"           -- [Optional] Framer type (lines, packet, none) / [可选] 帧处理器
    }
`;
    }

    if (selectedProtocols.value.includes('udp_server')) {
        script += `
    -- UDP Server Listener Configuration / UDP 服务端监听配置
    servers.udp = { 
        port = 9999                -- [Required] Listening port / [必填] 监听端口
    }
`;
    }

    if (selectedProtocols.value.includes('mqtt_client')) {
        script += `
    -- MQTT Client Configuration / MQTT 客户端配置
    clients.mqtt = {
        host = "127.0.0.1",      -- [Required] MQTT Broker Host / [必填] MQTT Broker 地址
        port = 1883,             -- [Required] MQTT Broker Port / [必填] MQTT Broker 端口
        client_id = "client-id", -- [Optional] Client ID / [可选] 客户端 ID
        username = "user",       -- [Optional] Username / [可选] 用户名
        password = "password",   -- [Optional] Password / [可选] 密码
        ssl = false,             -- [Optional] Enable SSL / [可选] 是否启用 SSL
        topics = { "v1/devices/+/telemetry", "v1/devices/+/attributes" }, -- [Required] Subscription Topics / [必填] 订阅主题
        qos = 1                  -- [Optional] Default QoS (0, 1, 2) / [可选] 默认 QoS 级别
    }
`;
    }

    if (selectedProtocols.value.includes('http_client')) {
        script += `
    -- HTTP Client Configuration / HTTP 客户端配置
    clients.http = {
        timeout = 5000           -- [Optional] Request timeout (ms) / [可选] 请求超时
    }
`;
    }

    if (selectedProtocols.value.includes('tcp_client')) {
        script += `
    -- TCP Client Configuration / TCP 客户端配置
    clients.tcp = {
        host = "127.0.0.1",      -- [Required] Host / [必填] 地址
        port = 8888,             -- [Required] Port / [必填] 端口
        framer = "lines"         -- [Optional] Framer type / [可选] 帧处理器
    }
`;
    }

    script += `
    return true, servers, clients
end

----------------------------------------------------------------
-- 1.1 Lifecycle: Start / 生命周期：启动
----------------------------------------------------------------
-- Called after listeners are started.
-- 在监听器启动后调用。
-- Useful for initial actions like login or subscribing to custom topics.
-- 用于执行初始化动作，如登录或订阅自定义主题。
-- @param ctx: Context object / 上下文对象
-- @return success (bool): true if successful / 是否成功
function on_start(ctx)
    ctx:log("info", "Plugin started...")
    
    -- [Example] MQTT Login / [示例] MQTT 登录
    -- mqtt.publish("v1/devices/login", json.encode({ token = "123" }))
    return true
end

----------------------------------------------------------------
-- 1.2 Lifecycle: Devices Initialization / 生命周期：设备初始化
----------------------------------------------------------------
-- Called after on_start to resolve real device identities.
-- 在 on_start 之后调用，用于解析真实的设备身份。
-- Useful for mapping device_code to dynamic external_id (e.g., from API).
-- 适用于将 device_code 映射到动态的 external_id（例如从 API 获取）。
-- @param ctx: Context object / 上下文对象
-- @return success (bool): true if successful / 是否成功
-- @return mapping (table): map of device_code -> external_id / 设备映射表
function on_devices_init(ctx)
    ctx:log("info", "Devices initializing...")

    -- [Example] Resolve external_id via API / [示例] 通过 API 解析 external_id
    -- local subs = ctx:get_sub_devices()
    -- local mapping = {}
    -- for _, sub in ipairs(subs) do
    --     local ip = sub.config.ip
    --     if ip then
    --         -- local real_id = http.get("http://subsystem/get_id?ip=" .. ip)
    --         -- mapping[sub.device_code] = real_id
    --     end
    -- end
    -- return true, mapping
    return true, {}
end
`;

    // 2. Lifecycle: Schedule Tasks
    script += `
----------------------------------------------------------------
-- 2. Lifecycle: Schedule Tasks / 生命周期：调度任务
----------------------------------------------------------------
-- Called to configure periodic tasks.
-- 用于配置周期性任务。
-- The system will call the defined functions at the specified intervals.
-- 系统将按照指定的时间间隔调用定义的函数。
-- @param ctx: Context object / 上下文对象
-- @return tasks (table): map of function_name -> ms / 任务配置表
function on_schedule(ctx)
    return {
        -- [Required] Global Heartbeat Timeout (ms). Default: 60s.
        -- [必填] 全局心跳超时时间 (毫秒)。默认: 60秒。
        heartbeat_timeout = 60 * 1000,

        -- [Optional] Child Device Heartbeat Timeout (ms). Default: same as heartbeat_timeout.
        -- [可选] 子设备心跳超时时间 (毫秒)。默认: 同全局超时。
        child_heartbeat_timeout = 60 * 1000,

        -- [Task 1] System Heartbeat (Active Probe) / 系统心跳（主动探测）
        on_system_heartbeat = 60 * 1000,

        -- [Task 2] System Poll (Data Collection) / 系统采集（数据采集）
        on_system_poll = 10 * 1000,

        -- [Task 3] Device Discovery / 设备发现
        on_device_discovery = 5 * 60 * 1000,

        -- [Task 4] Device Heartbeat / 设备心跳
        on_device_heartbeat = 30 * 1000,

        -- [Task 5] Device Poll / 设备采集
        on_device_poll = 5000,

        -- [Task 6] Token Refresh / Token刷新
        on_token_refresh = 60 * 60 * 1000
    }
end

----------------------------------------------------------------
-- 2.1 Lifecycle: Token Refresh / 生命周期：Token刷新
----------------------------------------------------------------
-- Called when the auth token is about to expire or needs refresh.
-- 当鉴权Token即将过期或需要刷新时调用。
-- @param ctx: Context
-- @return success (bool)
function on_token_refresh(ctx)
    ctx:log("info", "Token Refresh")
    
    -- [Example] Refresh Token
    -- local new_token = http.post(...)
    -- ctx:set("token", new_token)
    
    return true
end
`;

    // 3. System Tasks
    script += `
----------------------------------------------------------------
-- 3. System Tasks Implementation / 系统任务实现
----------------------------------------------------------------
-- Implementation of tasks defined in on_schedule.
-- 实现 on_schedule 中定义的任务。

-- [Callback] System Heartbeat / 系统心跳
-- @param ctx: Context
-- @return success (bool): true if system is healthy / 系统是否健康
function on_system_heartbeat(ctx)
    ctx:log("info", "Executing system heartbeat")
    return true
end

-- [Callback] System Poll / 系统采集
-- @param ctx: Context
-- @return data (table): system data object / 系统数据对象
-- Return Format:
-- {
--     properties = {               -- [Optional] Property Key-Value Map / [可选] 属性键值对
--         cpu_usage = 0.5,
--         memory_usage = 0.6
--     },
--     events = {                   -- [Optional] Event List / [可选] 事件列表
--         { id = "alarm", params = { msg = "High CPU" }, time = sys.now_ms() }
--     }
-- }
function on_system_poll(ctx)
    ctx:log("info", "Executing system poll")
    return {
        properties = {
            -- cpu_usage = 0.5
        }
    }
end
`;

    // 4. Device Tasks
    script += `
----------------------------------------------------------------
-- 4. Device Tasks Implementation / 设备任务实现
----------------------------------------------------------------
-- Device Discovery Task / 设备发现任务
-- Used to scan/list devices managed by the subsystem.
-- 用于扫描/列出子系统管理的设备。
-- @param ctx: Context
-- @return data_list (table): standard data list containing device info / 包含设备信息的标准数据列表
-- Return Format:
-- {
--     {
--         external_id = "device_01",   -- [Required] Device External ID / [必填] 设备外部ID
--         name = "Device 01",          -- [Optional] Device Name / [可选] 设备名称
--         product_code = "sensor_01",  -- [Optional] Product Code (if different from current) / [可选] 产品代码
--         properties = { ... }         -- [Optional] Initial Properties / [可选] 初始属性
--     }
-- }
function on_device_discovery(ctx)
    ctx:log("info", "Executing device discovery")
    return {}
end

-- Device Heartbeat Task / 设备心跳任务
-- Used to check online/offline status of devices.
-- 用于检查设备的在线/离线状态。
-- @param ctx: Context
-- @return data_list (table): standard data list with "online" field / 包含"online"字段的标准数据列表
-- Return Format:
-- {
--     {
--         external_id = "device_01",   -- [Required] Device External ID / [必填] 设备外部ID
--         online = true                -- [Required] Connection Status / [必填] 连接状态
--     }
-- }
function on_device_heartbeat(ctx)
    ctx:log("info", "Executing device heartbeat")
    local device_map = ctx:get_device_map()
    local data_list = {}
    for ext_id, code in pairs(device_map) do
        -- Perform heartbeat check for each sub-device
        -- table.insert(data_list, { external_id = ext_id, online = true })
    end
    return data_list
end

-- Device Poll Task / 设备采集任务
-- Used to actively fetch device data (properties/events).
-- 用于主动获取设备数据（属性/事件）。
-- @param ctx: Context
-- @return data_list (table): standard data list / 标准数据列表
-- Return Format:
-- {
--     {
--         external_id = "device_01",   -- [Required] Device External ID / [必填] 设备外部ID
--         properties = {               -- [Optional] Property Map / [可选] 属性表
--             temperature = 25.5,
--             humidity = 60
--         },
--         events = {                   -- [Optional] Event List / [可选] 事件列表
--             { id = "door_opened", params = {}, time = sys.now_ms() }
--         }
--     }
-- }
function on_device_poll(ctx)
    ctx:log("info", "Executing device poll")
    return {}
end
`;

    // 5. Control
    script += `
----------------------------------------------------------------
-- 5. Control (Downlink) / 控制（下行）
----------------------------------------------------------------

-- Set property for the Subsystem itself (Gateway Plugin) / 设置子系统本身属性
-- @param ctx: Context
-- @param values: Properties Table { key = value } / 属性表 (Multi-property write supported)
-- @return success (bool): communication success / 通信是否成功
-- @return results (table): granular results for each property (optional) / 每个属性的精细控制结果 (可选)
-- Return Table Format: { [prop_id] = { success = true, error = "msg" } }
function on_system_write_property(ctx, values)
    ctx:log("info", "System Write Prop: " .. json.encode(values))
    -- Example for granular feedback:
    -- return true, { cpu_limit = { success = true }, mem_limit = { success = false, error = "out of range" } }
    return true, {}
end

-- Call service for the Subsystem itself (Gateway Plugin) / 调用子系统本身服务
-- @param ctx: Context
-- @param service_id: Service ID (string) / 服务ID
-- @param params: Service Parameters (Table) / 服务参数
-- @return success (bool): operation success / 操作是否成功
-- @return result (table): result data map / 结果数据表
-- @return error_msg (string): error message if failed / 错误信息
function on_system_call_service(ctx, service_id, params)
    ctx:log("info", "System Call Service: " .. service_id)
    return true, {}, ""
end

-- Set property for a Child Device / 设置子设备属性
-- @param ctx: Context (Bound to the device) / 上下文（已绑定到设备）
-- @param values: Properties Table { key = value } / 属性表 (Multi-property write supported)
-- @return success (bool): communication success / 通信是否成功
-- @return results (table): granular results for each property (optional) / 每个属性的精细控制结果 (可选)
-- Return Table Format: { [prop_id] = { success = true, error = "msg" } }
function on_device_write_property(ctx, values)
    ctx:log("info", "Device Write Prop: " .. json.encode(values))
    return true, {}
end

-- Call service for a Child Device / 调用子设备服务
-- @param ctx: Context (Bound to the device) / 上下文（已绑定到设备）
-- @param service_id: Service ID / 服务ID
-- @param params: Service Parameters (Table) / 服务参数
-- @return success (bool): operation success / 操作是否成功
-- @return result (table): result data map / 结果数据表
-- @return error_msg (string): error message if failed / 错误信息
function on_device_call_service(ctx, service_id, params)
    ctx:log("info", "Device Call Service: " .. service_id)
    return true, {}, ""
end
`;

    // 6. Listeners
    if (selectedProtocols.value.length > 0) {
        script += `
----------------------------------------------------------------
-- 6. Protocol Listeners (Push/Active) / 协议监听（主动推送）
----------------------------------------------------------------
`;
        if (selectedProtocols.value.includes('http_server')) {
            script += `
-- Return Format:
-- {
--     {
--         external_id = "sensor_01",   -- [Required] Device External ID / [必填] 设备外部ID
--         online = true,               -- [Optional] Connection Status / [可选] 连接状态
--         properties = {               -- [Optional] Property Key-Value Map / [可选] 属性键值对
--             temperature = 25.5
--         },
--         events = {                   -- [Optional] Event List / [可选] 事件列表
--             { id = "motion", params = { value = 1 }, time = sys.now_ms() }
--         }
--     }
-- }
function on_http_handle(ctx, req)
    ctx:log("info", "HTTP Request: " .. req.path)
    
    -- [Example] Handle Alarm Webhook / [示例] 处理报警Webhook
    -- if req.path == "/alarm" then
    --     local msg = json.decode(req.body)
    --     return { code = 200, body = "OK" }, {
    --         {
    --             external_id = msg.device_id,
    --             online = true, -- Optional status update
    --             events = { { id = "alarm", params = msg.data, time = sys.now_ms() } }
    --         }
    --     }
    -- end

    return { code = 200, body = "OK" }, {}
end
`;
        }

        if (selectedProtocols.value.includes('tcp_server')) {
            script += `
-- Handle TCP Message / 处理TCP消息
-- @param ctx: Context
-- @param data: string (Received binary/text data / 接收到的原始数据)
-- @param client_ip: string (Source IP and port / 来源IP和端口)
-- @return reply: string (Data to send back to client, use nil for no reply / 返回给客户端的数据，nil表示不回复)
-- @return data_list: table (optional, data to report to gateway / 上报给网关的数据列表)
function on_tcp_handle(ctx, data, client_ip)
    ctx:log("info", "TCP Message from " .. client_ip)
    return nil, {}
end
`;
        }

        if (selectedProtocols.value.includes('udp_server')) {
            script += `
-- Handle UDP Message / 处理UDP消息
-- @param ctx: Context
-- @param data: string (Received data / 接收到的数据)
-- @param client_addr: string (Source address / 来源地址)
-- @return reply: string (Data to send back to sender / 返回给发送者的数据)
-- @return data_list: table (optional, data to report to gateway / 上报给网关的数据列表)
function on_udp_handle(ctx, data, client_addr)
    ctx:log("info", "UDP Message from " .. client_addr)
    return nil, {}
end
`;
        }
        
        if (selectedProtocols.value.includes('mqtt_client')) {
            script += `
-- [Callback] MQTT Message / MQTT消息处理
-- @param ctx: Context
-- @param topic: string (Message topic / 消息主题)
-- @param payload: string (Message body / 消息内容)
-- @return reply: string (Not used for now, reserved for Req-Res / 暂未使用，预留用于请求-响应模式)
-- @return data_list: table (optional, same format as on_http_handle / 上报给网关的数据列表)
function on_mqtt_handle(ctx, topic, payload)
    ctx:log("info", "MQTT Msg: " .. topic)
    return nil, {}
end
`;
        }
    }

    // 7. Status Reporting
    script += `
----------------------------------------------------------------
-- 7. Status Reporting Examples / 状态上报示例
----------------------------------------------------------------
-- You can report status manually using ctx:report_status()
-- 你可以使用 ctx:report_status() 手动上报状态。
-- This is useful when you want to report status outside of the return value mechanism.
-- 当你想在返回值机制之外上报状态时，这很有用。

-- [Example] Report current device status / [示例] 上报当前设备状态
-- ctx:report_status("online")

-- [Example] Batch report sub-device status / [示例] 批量上报子设备状态
-- ctx:report_status({
--     ["ext_id_01"] = "online",
--     ["ext_id_02"] = "offline"
-- })
`;

    // 8. Destroy
    script += `
----------------------------------------------------------------
-- 8. Destroy / 销毁
----------------------------------------------------------------
-- Called when the plugin is destroyed (disabled, removed, or updated).
-- 当插件被销毁（禁用、移除或更新）时调用。
-- Used to release resources (e.g., close custom connections).
-- 用于释放资源（例如关闭自定义连接）。
-- @param ctx: Context
function on_destroy(ctx)
    ctx:log("info", "Plugin destroyed")
    -- [Example] Release resources / [示例] 释放资源
    -- if my_connection then my_connection:close() end
end
`;

    // Simulate manual edit by calling updateParams with the new script
    // scriptContent.value = script; // Removed to rely on round-trip update
    updateParams(script);

    activeTab.value = 'script';
  } catch (e) {
    console.error(e);
    alert('Failed to generate script');
  }
};

const copyScript = () => {
    if (!scriptContent.value) return;
    navigator.clipboard.writeText(scriptContent.value);
};

const downloadScript = () => {
    if (!scriptContent.value) return;
    const blob = new Blob([scriptContent.value], { type: 'text/plain' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'driver.lua';
    a.click();
    URL.revokeObjectURL(url);
};

const startDebug = () => {
    showDeviceSelect.value = true;
    loadDevices();
};

const loadDevices = async () => {
    try {
        // Fetch devices that use this product or script plugin
        // For simplicity, fetch all and filter client side or mock
        // Ideally: /api/devices?product_code=...
        const res = await axios.get('/api/devices', { params: { product_code: props.productCode } });
        if (res.data.code === 200) {
            devices.value = res.data.data.list || [];
        }
    } catch (e) {
        console.error(e);
    }
};

const selectDevice = (d) => {
    selectedDeviceCode.value = d.code;
    showDeviceSelect.value = false;
    showDebug.value = true;
};
</script>

<style scoped>
.script-editor-container {
    border: 1px solid #dee2e6;
    border-radius: 4px;
    overflow: hidden;
}

.editor-fullscreen {
    position: fixed;
    top: 0;
    left: 0;
    width: 100vw;
    height: 100vh;
    background: white;
    z-index: 1050;
    padding: 10px;
}
</style>
