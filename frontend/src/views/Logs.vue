<template>
  <div class="logs-container h-100 d-flex flex-column">
    <div class="d-flex justify-content-between align-items-center mb-3">
      <h4 class="mb-0">{{ $t('sidebar_logs') }}</h4>
      
      <div class="btn-group">
        <button class="btn btn-outline-primary" :class="{ active: activeTab === 'realtime' }" @click="activeTab = 'realtime'">
          <i class="bi bi-activity me-1"></i> {{ $t('realtime_logs') }}
        </button>
        <button class="btn btn-outline-primary" :class="{ active: activeTab === 'history' }" @click="activeTab = 'history'">
          <i class="bi bi-clock-history me-1"></i> {{ $t('history_logs') }}
        </button>
      </div>
    </div>

    // Real-time Logs
    <div v-if="activeTab === 'realtime'" class="card border-0 shadow-sm flex-grow-1 d-flex flex-column overflow-hidden">
      <div class="card-header bg-transparent d-flex justify-content-between align-items-center">
        <div class="d-flex align-items-center gap-3">
          <button class="btn btn-sm" :class="isPaused ? 'btn-success' : 'btn-warning'" @click="togglePause">
            <i class="bi" :class="isPaused ? 'bi-play-fill' : 'bi-pause-fill'"></i>
            {{ isPaused ? $t('btn_resume') : $t('btn_pause') }}
          </button>
          <button class="btn btn-sm btn-outline-secondary" @click="clearLogs">
            <i class="bi bi-trash"></i> {{ $t('btn_clear') }}
          </button>
          <span class="badge" :class="wsConnected ? 'bg-success' : 'bg-danger'">
            {{ wsConnected ? $t('ws_connected') : $t('ws_disconnected') }}
          </span>
        </div>
        <div class="d-flex align-items-center gap-2">
          <div class="btn-group btn-group-sm me-2">
            <input type="checkbox" class="btn-check" id="lvl-debug" value="debug" v-model="filterLevels">
            <label class="btn btn-outline-secondary" for="lvl-debug">Debug</label>

            <input type="checkbox" class="btn-check" id="lvl-info" value="info" v-model="filterLevels">
            <label class="btn btn-outline-info" for="lvl-info">Info</label>

            <input type="checkbox" class="btn-check" id="lvl-warn" value="warn" v-model="filterLevels">
            <label class="btn btn-outline-warning" for="lvl-warn">Warn</label>

            <input type="checkbox" class="btn-check" id="lvl-error" value="error" v-model="filterLevels">
            <label class="btn btn-outline-danger" for="lvl-error">Error</label>
          </div>
          <div class="d-flex align-items-center gap-2">
            <input type="text" class="form-control form-control-sm" :placeholder="$t('filter_text')" v-model="filterText" style="width: 150px;">
            <div class="input-group input-group-sm" style="width: 320px;">
              <input type="text" class="form-control" :placeholder="$t('search_text')" v-model="searchText" @keyup.enter="nextMatch('rt')">
              <button class="btn btn-outline-secondary" type="button" @click="prevMatch('rt')" :disabled="!searchText"><i class="bi bi-chevron-up"></i></button>
              <button class="btn btn-outline-secondary" type="button" @click="nextMatch('rt')" :disabled="!searchText"><i class="bi bi-chevron-down"></i></button>
              <span class="input-group-text px-2 text-muted" style="font-size: 0.75rem;" v-if="searchText">
                {{ rtMatchCount > 0 ? rtCurrentMatch + 1 + '/' + rtMatchCount : '0/0' }}
              </span>
              <input type="checkbox" class="btn-check" id="rt-match-case" v-model="rtMatchCase">
              <label class="btn btn-outline-secondary fw-bold" for="rt-match-case" :title="$t('match_case')">Aa</label>
              <input type="checkbox" class="btn-check" id="rt-match-word" v-model="rtMatchWord">
              <label class="btn btn-outline-secondary fw-bold" for="rt-match-word" :title="$t('match_whole_word')">\b</label>
            </div>
          </div>
        </div>
      </div>
      <div class="card-body p-0 bg-body-tertiary font-monospace overflow-auto" ref="logContainer" style="font-size: 13px;">
        <div class="p-3">
          <div v-for="(log, idx) in filteredLogs" :key="idx" class="log-line mb-1" :class="getLogColor(log.level)">
            <span class="text-secondary">[{{ log.time }}]</span>
            <span class="fw-bold mx-2">[{{ log.level.toUpperCase() }}]</span>
            <span v-if="log.logger" class="text-info" v-html="`[${highlightText(log.logger, searchText, rtMatchCase, rtMatchWord)}]`"></span>
            <span v-html="highlightText(log.message, searchText, rtMatchCase, rtMatchWord)"></span>
            <span v-if="log.fields && Object.keys(log.fields).length > 0" class="text-secondary ms-2" v-html="highlightText(JSON.stringify(log.fields), searchText, rtMatchCase, rtMatchWord)"></span>
          </div>
        </div>
      </div>
    </div>

    <!-- History Logs -->
    <div v-if="activeTab === 'history'" class="card border-0 shadow-sm flex-grow-1 d-flex flex-column overflow-hidden">
      <div class="card-body p-0 d-flex h-100">
        <!-- File List Sidebar -->
        <div class="border-end d-flex flex-column" style="width: 280px;">
          <div class="p-3 border-bottom d-flex justify-content-between align-items-center">
            <h6 class="mb-0">{{ $t('log_files') }}</h6>
            <button class="btn btn-sm btn-outline-secondary" @click="fetchFiles" :title="$t('btn_refresh')">
              <i class="bi bi-arrow-clockwise"></i>
            </button>
          </div>
          <div class="flex-grow-1 overflow-auto">
            <div v-if="loadingFiles" class="text-center py-3">
              <div class="spinner-border spinner-border-sm text-primary" role="status"></div>
            </div>
            <div v-else-if="files.length === 0" class="text-center py-3 text-muted small">
              {{ $t('no_log_files') }}
            </div>
            <div class="list-group list-group-flush rounded-0">
              <button v-for="file in files" :key="file.name" 
                class="list-group-item list-group-item-action d-flex justify-content-between align-items-start"
                :class="{ active: currentFile === file.name }"
                @click="selectFile(file.name)">
                <div class="ms-2 me-auto text-truncate">
                  <div class="fw-bold text-truncate" style="font-size: 14px;">{{ formatLogName(file) }}</div>
                  <small :class="currentFile === file.name ? 'text-white' : 'text-muted'">{{ formatSize(file.size) }}</small>
                </div>
                <small :class="currentFile === file.name ? 'text-white' : 'text-muted'">{{ file.time.split(' ')[0] }}</small>
              </button>
            </div>
          </div>
        </div>

        <!-- File Content View -->
        <div class="flex-grow-1 d-flex flex-column bg-body-tertiary position-relative">
          <div v-if="currentFile" class="p-2 border-bottom d-flex justify-content-between align-items-center bg-body-secondary">
            <span class="font-monospace small">{{ currentFile }}</span>
            <div class="d-flex align-items-center gap-2">
              <div class="d-flex align-items-center gap-2">
                <input type="text" class="form-control form-control-sm" :placeholder="$t('filter_text')" v-model="historyFilterText" style="width: 150px;">
                <div class="input-group input-group-sm" style="width: 320px;">
                  <input type="text" class="form-control" :placeholder="$t('search_text')" v-model="historySearchText" @keyup.enter="nextMatch('hist')">
                  <button class="btn btn-outline-secondary" type="button" @click="prevMatch('hist')" :disabled="!historySearchText"><i class="bi bi-chevron-up"></i></button>
                  <button class="btn btn-outline-secondary" type="button" @click="nextMatch('hist')" :disabled="!historySearchText"><i class="bi bi-chevron-down"></i></button>
                  <span class="input-group-text px-2 text-muted" style="font-size: 0.75rem;" v-if="historySearchText">
                    {{ histMatchCount > 0 ? histCurrentMatch + 1 + '/' + histMatchCount : '0/0' }}
                  </span>
                  <input type="checkbox" class="btn-check" id="hist-match-case" v-model="histMatchCase">
                  <label class="btn btn-outline-secondary fw-bold" for="hist-match-case" :title="$t('match_case')">Aa</label>
                  <input type="checkbox" class="btn-check" id="hist-match-word" v-model="histMatchWord">
                  <label class="btn btn-outline-secondary fw-bold" for="hist-match-word" :title="$t('match_whole_word')">\b</label>
                </div>
              </div>
              <a :href="`/api/system/log/download?name=${currentFile}`" target="_blank" class="btn btn-sm btn-outline-primary text-nowrap">
                <i class="bi bi-download"></i> {{ $t('btn_download') }}
              </a>
            </div>
          </div>
          <div class="flex-grow-1 overflow-auto p-3 font-monospace" ref="histLogContainer" style="font-size: 13px; white-space: pre-wrap;">
            <div v-if="loadingContent" class="text-center py-5">
              <div class="spinner-border text-primary" role="status"></div>
            </div>
            <div v-else-if="!currentFile" class="h-100 d-flex align-items-center justify-content-center text-muted">
              {{ $t('select_file_view') }}
            </div>
            <div v-else-if="filteredFileContent === '' && historyFilterText" class="h-100 d-flex align-items-center justify-content-center text-muted">
              {{ $t('no_search_results') }}
            </div>
            <div v-else v-html="highlightText(filteredFileContent, historySearchText, histMatchCase, histMatchWord)"></div>
          </div>
        </div>
      </div>
    </div>

    <!-- AI Tooltip -->
    <div v-if="showAiTooltip" class="position-fixed" :style="{ top: tooltipY + 'px', left: tooltipX + 'px', zIndex: 1050 }">
      <button class="btn btn-sm btn-primary shadow rounded-pill" @click.stop="openAiAnalysis">
        <i class="bi bi-robot"></i> {{ $t('ai_analyze') || 'AI 分析' }}
      </button>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from 'vue';
import axios from 'axios';
import { inject } from 'vue';
import { useI18n } from 'vue-i18n';

const { t } = useI18n();
const showToast = inject('showToast');

const activeTab = ref('realtime'); // realtime | history

// Real-time state
const logs = ref([]);
const isPaused = ref(false);
const wsConnected = ref(false);
const filterLevels = ref(['debug', 'info', 'warn', 'error']);
const filterText = ref('');
const searchText = ref('');
const rtMatchCase = ref(false);
const rtMatchWord = ref(false);
const logContainer = ref(null);
const rtMatchCount = ref(0);
const rtCurrentMatch = ref(0);
const maxLogs = 1000;
let ws = null;

// History state
const files = ref([]);
const loadingFiles = ref(false);
const currentFile = ref('');
const fileContent = ref('');
const loadingContent = ref(false);
const historyFilterText = ref('');
const historySearchText = ref('');
const histMatchCase = ref(false);
const histMatchWord = ref(false);
const histLogContainer = ref(null);
const histMatchCount = ref(0);
const histCurrentMatch = ref(0);

// AI Analysis Selection State
const showAiTooltip = ref(false);
const tooltipX = ref(0);
const tooltipY = ref(0);
const selectedLogText = ref('');

const handleSelection = (e) => {
  setTimeout(() => {
    const selection = window.getSelection();
    const text = selection.toString().trim();

    if (text && text.length > 5) { // Only show for meaningful selections
      // Ensure the selection is within the logs container
      let isLogSelection = false;
      let node = selection.anchorNode;
      while (node) {
        if (node.classList && (node.classList.contains('log-line') || node.classList.contains('font-monospace'))) {
          isLogSelection = true;
          break;
        }
        node = node.parentNode;
      }

      if (isLogSelection) {
        const range = selection.getRangeAt(0);
        const rect = range.getBoundingClientRect();
        
        // Position tooltip near the mouse cursor or the end of selection
        tooltipX.value = e.clientX + 10;
        tooltipY.value = e.clientY + 15;
        
        // Prevent tooltip from going off-screen
        if (tooltipX.value + 100 > window.innerWidth) {
          tooltipX.value = window.innerWidth - 100;
        }
        if (tooltipY.value + 40 > window.innerHeight) {
          tooltipY.value = window.innerHeight - 40;
        }
        
        selectedLogText.value = text;
        showAiTooltip.value = true;
        return;
      }
    }
    
    // Hide if no valid selection or clicked elsewhere
    showAiTooltip.value = false;
  }, 10);
};

const handleGlobalClick = (e) => {
  // Hide tooltip if clicked outside
  if (showAiTooltip.value) {
    showAiTooltip.value = false;
  }
};

const openAiAnalysis = () => {
  showAiTooltip.value = false;
  window.dispatchEvent(new CustomEvent('noyo-analyze-log', {
    detail: {
      text: selectedLogText.value
    }
  }));
};

const escapeHtml = (unsafe) => {
  if (!unsafe) return '';
  return unsafe.toString()
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#039;");
};

const highlightText = (text, query, matchCase, matchWord) => {
  if (!text) return '';
  if (!query) return escapeHtml(text);
  
  const escapedQ = query.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const flags = matchCase ? 'g' : 'gi';
  const regexStr = matchWord ? `\\b(${escapedQ})\\b` : `(${escapedQ})`;
  
  try {
    const regex = new RegExp(regexStr, flags);
    const parts = text.toString().split(regex);
    
    return parts.map((part, i) => {
      if (i % 2 === 1) { // Matched part
        return `<mark class="bg-warning text-dark px-1 rounded log-match">${escapeHtml(part)}</mark>`;
      } else {
        return escapeHtml(part);
      }
    }).join('');
  } catch (e) {
    return escapeHtml(text);
  }
};

// Computed filtered logs
const filteredLogs = computed(() => {
  return logs.value.filter(log => {
    if (filterLevels.value.length > 0 && !filterLevels.value.includes(log.level.toLowerCase())) return false;
    if (filterText.value) {
      const q = filterText.value.toLowerCase();
      
      const inMessage = log.message && log.message.toLowerCase().includes(q);
      const inLogger = log.logger && log.logger.toLowerCase().includes(q);
      const inFields = log.fields && Object.keys(log.fields).length > 0 
        ? JSON.stringify(log.fields).toLowerCase().includes(q) 
        : false;
        
      return inMessage || inLogger || inFields;
    }
    return true;
  });
});

const filteredFileContent = computed(() => {
  if (!fileContent.value) return '';
  if (!historyFilterText.value) return fileContent.value;
  
  const q = historyFilterText.value.toLowerCase();
  
  return fileContent.value.split('\n')
    .filter(line => line.toLowerCase().includes(q))
    .join('\n');
});

const getLogColor = (level) => {
  switch (level.toLowerCase()) {
    case 'error': return 'text-danger';
    case 'warn': return 'text-warning';
    case 'info': return 'text-info';
    case 'debug': return 'text-secondary';
    default: return 'text-body';
  }
};

const togglePause = () => {
  isPaused.value = !isPaused.value;
};

const clearLogs = () => {
  logs.value = [];
};

const highlightCurrentMatch = (matches, index, scroll = false) => {
  matches.forEach((el, i) => {
    if (i === index) {
      el.classList.remove('bg-warning', 'text-dark');
      el.classList.add('bg-danger', 'text-white');
      if (scroll) {
        el.scrollIntoView({ behavior: 'smooth', block: 'center' });
      }
    } else {
      el.classList.remove('bg-danger', 'text-white');
      el.classList.add('bg-warning', 'text-dark');
    }
  });
};

const updateMatchCount = async (type, scroll = false) => {
  await nextTick();
  if (type === 'rt') {
    if (!logContainer.value) return;
    const matches = logContainer.value.querySelectorAll('.log-match');
    rtMatchCount.value = matches.length;
    if (rtMatchCount.value > 0) {
      if (rtCurrentMatch.value >= rtMatchCount.value) {
        rtCurrentMatch.value = 0;
      }
      highlightCurrentMatch(matches, rtCurrentMatch.value, scroll);
    } else {
      rtCurrentMatch.value = 0;
    }
  } else {
    if (!histLogContainer.value) return;
    const matches = histLogContainer.value.querySelectorAll('.log-match');
    histMatchCount.value = matches.length;
    if (histMatchCount.value > 0) {
      if (histCurrentMatch.value >= histMatchCount.value) {
        histCurrentMatch.value = 0;
      }
      highlightCurrentMatch(matches, histCurrentMatch.value, scroll);
    } else {
      histCurrentMatch.value = 0;
    }
  }
};

const nextMatch = (type) => {
  if (type === 'rt' && rtMatchCount.value > 0) {
    rtCurrentMatch.value = (rtCurrentMatch.value + 1) % rtMatchCount.value;
    updateMatchCount('rt', true);
  } else if (type === 'hist' && histMatchCount.value > 0) {
    histCurrentMatch.value = (histCurrentMatch.value + 1) % histMatchCount.value;
    updateMatchCount('hist', true);
  }
};

const prevMatch = (type) => {
  if (type === 'rt' && rtMatchCount.value > 0) {
    rtCurrentMatch.value = (rtCurrentMatch.value - 1 + rtMatchCount.value) % rtMatchCount.value;
    updateMatchCount('rt', true);
  } else if (type === 'hist' && histMatchCount.value > 0) {
    histCurrentMatch.value = (histCurrentMatch.value - 1 + histMatchCount.value) % histMatchCount.value;
    updateMatchCount('hist', true);
  }
};

watch(filteredLogs, () => {
  if (searchText.value) {
    updateMatchCount('rt', false);
  }
});

watch([searchText, rtMatchCase, rtMatchWord], () => {
  rtCurrentMatch.value = 0;
  if (searchText.value) {
    updateMatchCount('rt', true);
  } else {
    rtMatchCount.value = 0;
  }
});

watch(filteredFileContent, () => {
  if (historySearchText.value) {
    updateMatchCount('hist', false);
  }
});

watch([historySearchText, histMatchCase, histMatchWord], () => {
  histCurrentMatch.value = 0;
  if (historySearchText.value) {
    updateMatchCount('hist', true);
  } else {
    histMatchCount.value = 0;
  }
});

const scrollToBottom = async () => {
  if (isPaused.value) return;
  await nextTick();
  if (logContainer.value) {
    logContainer.value.scrollTop = logContainer.value.scrollHeight;
  }
};

const fetchRealtimeTail = async () => {
  try {
    const res = await axios.get('/api/system/log/tail?lines=100');
    if (res.data.code === 0 && res.data.data) {
      logs.value = res.data.data;
      scrollToBottom();
      // Also update search matches if there's a search query
      if (searchText.value) {
        updateMatchCount('rt');
      }
    }
  } catch (e) {
    console.error('Failed to fetch tail logs:', e);
  }
};

const connectWebSocket = () => {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const host = window.location.host;
  ws = new WebSocket(`${protocol}//${host}/api/system/log/stream`);

  ws.onopen = () => {
    wsConnected.value = true;
  };

  ws.onmessage = (event) => {
    if (isPaused.value) return;
    
    try {
      const data = JSON.parse(event.data);
      if (data.type === 'sys') {
        // System message
        return;
      }
      
      logs.value.push(data);
      if (logs.value.length > maxLogs) {
        logs.value.shift();
      }
      scrollToBottom();
    } catch (e) {
      // Not JSON
    }
  };

  ws.onclose = () => {
    wsConnected.value = false;
    // Try to reconnect if tab is still active
    if (activeTab.value === 'realtime') {
      setTimeout(connectWebSocket, 3000);
    }
  };

  ws.onerror = () => {
    wsConnected.value = false;
  };
};

const disconnectWebSocket = () => {
  if (ws) {
    ws.close();
    ws = null;
  }
};

// History methods
const fetchFiles = async () => {
  loadingFiles.value = true;
  try {
    const res = await axios.get('/api/system/log/files');
    if (res.data.code === 0) {
      files.value = res.data.data;
    }
  } catch (e) {
    showToast('danger', t('load_fail'));
  } finally {
    loadingFiles.value = false;
  }
};

const formatSize = (bytes) => {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
};

const formatLogName = (file) => {
  if (file.name === 'noyo.log') return t('today') || 'Today';
  return file.time.split(' ')[0];
};

const selectFile = async (name) => {
  currentFile.value = name;
  loadingContent.value = true;
  fileContent.value = '';
  try {
    const res = await axios.get(`/api/system/log/file?name=${name}`);
    if (res.data.code === 0) {
      fileContent.value = res.data.data;
    } else {
      fileContent.value = t('content_fail') + res.data.message;
    }
  } catch (e) {
    fileContent.value = t('content_fail') + e.message;
  } finally {
    loadingContent.value = false;
  }
};

watch(activeTab, (newTab) => {
  if (newTab === 'realtime') {
    if (!wsConnected.value) {
      fetchRealtimeTail().then(() => {
        connectWebSocket();
      });
    }
  } else {
    disconnectWebSocket();
    if (files.value.length === 0) fetchFiles();
  }
});

onMounted(() => {
  fetchRealtimeTail().then(() => {
    connectWebSocket();
  });
  document.addEventListener('mouseup', handleSelection);
  document.addEventListener('click', handleGlobalClick);
});

onUnmounted(() => {
  disconnectWebSocket();
  document.removeEventListener('mouseup', handleSelection);
  document.removeEventListener('click', handleGlobalClick);
});
</script>

<style scoped>
.logs-container {
  /* Subtract padding/margins if necessary to fit perfectly */
  height: calc(100vh - 120px);
}
.log-line {
  word-break: break-all;
}
.log-line:hover {
  background-color: rgba(255,255,255,0.05);
}
</style>
