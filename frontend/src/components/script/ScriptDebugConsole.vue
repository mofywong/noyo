<template>
  <div class="modal fade show d-block" style="background: rgba(0,0,0,0.5); z-index: 1060;">
    <div class="modal-dialog modal-xl">
      <div class="modal-content h-100 shadow-lg">
        <div class="modal-header bg-dark text-white border-bottom border-secondary">
          <h5 class="modal-title font-monospace"><i class="bi bi-bug me-2"></i>{{ $t('script_debug_console') }} <span class="text-muted small">({{ deviceCode }})</span></h5>
          <button type="button" class="btn-close btn-close-white" @click="$emit('close')"></button>
        </div>
        <div class="modal-body d-flex flex-column bg-dark text-white p-0" style="height: 80vh;">
          <!-- Toolbar -->
          <div class="p-2 border-bottom border-secondary d-flex gap-2 align-items-center bg-secondary bg-opacity-25">
            <div class="input-group input-group-sm" style="width: 250px;">
              <span class="input-group-text bg-secondary text-white border-0">Function</span>
              <input type="text" class="form-control bg-dark text-white border-secondary" v-model="funcName" placeholder="e.g. on_init">
            </div>
            <div class="input-group input-group-sm flex-grow-1">
              <span class="input-group-text bg-secondary text-white border-0">Args (Lua)</span>
              <input type="text" class="form-control bg-dark text-white border-secondary" v-model="funcArgs" placeholder="e.g. {host='1.2.3.4'}">
            </div>
            <button class="btn btn-sm btn-success" @click="runDebug" :disabled="running">
              <i class="bi bi-play-fill me-1"></i> Run
            </button>
             <button class="btn btn-sm btn-danger" @click="stopDebug" :disabled="!running">
              <i class="bi bi-stop-fill me-1"></i> Stop
            </button>
          </div>
          
          <!-- Output -->
          <div class="flex-grow-1 p-3 font-monospace overflow-auto" style="white-space: pre-wrap; background-color: #1e1e1e;">
            <div v-for="(log, i) in logs" :key="i" :class="getLogClass(log.level)" class="mb-1">
              <span class="text-secondary opacity-50 me-2">[{{ log.time }}]</span>
              <span>{{ log.msg }}</span>
            </div>
            <div v-if="logs.length === 0" class="text-secondary opacity-50 text-center mt-5">
              Ready to debug. Enter function name and arguments, then click Run.
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue';
import axios from 'axios';

const props = defineProps({
    deviceCode: { type: String, required: true }
});
const emit = defineEmits(['close']);

const funcName = ref('on_init');
const funcArgs = ref('{}');
const logs = ref([]);
const running = ref(false);
let ws = null;

const getLogClass = (level) => {
  switch(level) {
    case 'error': return 'text-danger';
    case 'warn': return 'text-warning';
    case 'info': return 'text-info';
    case 'success': return 'text-success fw-bold';
    default: return 'text-light';
  }
};

const appendLog = (level, msg) => {
  logs.value.push({
    time: new Date().toLocaleTimeString(),
    level,
    msg
  });
};

const connect = () => {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const host = window.location.host;
    // Connect to specific debug endpoint
    ws = new WebSocket(`${protocol}//${host}/api/plugins/script/debug/attach?device_code=${props.deviceCode}`);
    
    ws.onopen = () => {
        appendLog('info', 'Debugger connected.');
    };
    
    ws.onmessage = (event) => {
        try {
            const data = JSON.parse(event.data);
            if (data.type === 'log') {
                appendLog(data.level, data.message);
            } else if (data.type === 'error') {
                appendLog('error', 'Error: ' + data.message);
            }
        } catch (e) {
            appendLog('info', event.data);
        }
    };
    
    ws.onclose = () => {
        appendLog('warn', 'Debugger disconnected.');
        running.value = false;
    };
    
    ws.onerror = (e) => {
        appendLog('error', 'WebSocket error. Ensure backend is running.');
        running.value = false;
    };
};

const runDebug = async () => {
    if (!ws || ws.readyState !== WebSocket.OPEN) {
        connect();
    }
    
    running.value = true;
    appendLog('info', `Running ${funcName.value}...`);

    try {
        // Construct Lua script: return func(args)
        const script = `return ${funcName.value}(${funcArgs.value})`;
        
        const res = await axios.post('/api/plugins/script/debug/eval', null, {
            params: {
                device_code: props.deviceCode,
                script: script
            }
        });

        if (res.data.result) {
            appendLog('success', 'Result: ' + res.data.result);
        } else if (res.data.error) {
            appendLog('error', 'Error: ' + res.data.error);
        }
    } catch (e) {
        appendLog('error', 'Request failed: ' + e.message);
    } finally {
        running.value = false;
    }
};

const stopDebug = () => {
    // There is no "stop" API yet, but we can close WS to simulate detach
    if (ws) ws.close();
    running.value = false;
    setTimeout(connect, 500); // Reconnect
};

onMounted(() => {
    connect();
});

onUnmounted(() => {
    if (ws) ws.close();
});
</script>
