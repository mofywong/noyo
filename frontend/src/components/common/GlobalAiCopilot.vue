<template>
  <div class="global-ai-copilot">
    <!-- Floating Button -->
    <div 
      class="ai-fab-container"
      :style="{ right: position.right + 'px', bottom: position.bottom + 'px' }"
      ref="fabContainer"
    >
      <button
        class="ai-fab"
        @mousedown="startDrag($event, 'fab')"
        @click="handleClick"
        :title="$t('ai_copilot') || 'AI 助手'"
      >
        <i class="bi bi-robot icon-pulse"></i>
      </button>

      <!-- Promo Tooltip -->
      <transition name="fade-slide">
        <div v-if="showPromo" class="ai-promo-tooltip shadow text-start">
          <div class="d-flex justify-content-between align-items-center mb-1">
             <strong><i class="bi bi-stars text-warning me-1"></i> AI 智能助手</strong>
             <button type="button" class="btn-close btn-close-white" style="font-size: 0.5rem" @click.stop="closePromo"></button>
          </div>
          <div class="small fw-light">
             {{ promoMessage }}
          </div>
        </div>
      </transition>
    </div>

    <!-- Chat Window -->
    <div
      v-if="isOpen"
      class="ai-chat-window"
      :style="dialogPositionStyle"
      :class="[{ 'ai-chat-expanded': isExpanded, 'is-resizing': isResizing }, isDarkMode ? 'ai-window-dark' : 'ai-window-light']"
    >
      <div
        class="card-header ai-drag-handle d-flex justify-content-between align-items-center py-2 px-3"
        :class="isDarkMode ? 'ai-header-dark' : 'ai-header-light'"
        @mousedown="startDrag($event, 'window')"
      >
        <div class="d-flex align-items-center gap-2">
          <div class="ai-avatar">
            <i class="bi bi-stars"></i>
          </div>
          <div>
            <h6 class="mb-0 fw-semibold" :class="isDarkMode ? 'text-white' : 'text-dark'">{{ $t('ai_copilot') }}</h6>
            <small class="ai-status-text" :class="isDarkMode ? 'text-light opacity-75' : 'text-muted'">
              <span class="ai-status-dot"></span> {{ isTyping ? $t('ai_thinking') : $t('ai_online_status') }}
            </small>
          </div>
        </div>
        <div class="d-flex align-items-center gap-1">
           <button class="btn btn-sm ai-btn-icon" @click="createNewSession" :title="$t('ai_new_chat')">
              <i class="bi bi-plus-lg"></i>
           </button>
           <button class="btn btn-sm ai-btn-icon" @click="showHistory = !showHistory" :title="$t('ai_history')">
              <i class="bi bi-clock-history"></i>
           </button>
           <button class="btn btn-sm ai-btn-icon" @click="toggleExpand" :title="isExpanded ? $t('collapse') : $t('expand')">
              <i class="bi" :class="isExpanded ? 'bi-arrows-angle-contract' : 'bi-arrows-angle-expand'"></i>
           </button>
           <button class="btn btn-sm ai-btn-icon ai-btn-close" @click="toggleChat">
              <i class="bi bi-x-lg"></i>
           </button>
        </div>
      </div>
      
      <!-- History Area -->
      <div v-if="showHistory" class="card-body bg-light overflow-auto flex-grow-1 p-0">
          <div class="list-group list-group-flush">
             <div v-for="session in sessions" :key="session.id" 
                  class="list-group-item list-group-item-action d-flex justify-content-between align-items-center cursor-pointer"
                  :class="{'bg-primary text-white': session.id === currentSessionId}"
                  @click="selectSession(session.id)">
                <span class="text-truncate" style="max-width: 80%">
                   <i class="bi bi-chat-left-text me-2"></i>{{ session.title }}
                </span>
                <button class="btn btn-sm btn-outline-danger border-0" :class="{'text-white': session.id === currentSessionId}" @click.stop="deleteSession(session.id)">
                   <i class="bi bi-trash"></i>
                </button>
             </div>
          </div>
          <div v-if="sessions.length === 0" class="text-center text-muted p-4">{{ $t('ai_no_history') }}</div>
      </div>

      <!-- Messages Area -->
      <div v-else class="card-body chat-messages overflow-auto flex-grow-1 p-3" ref="messagesContainer">
        <div v-if="currentMessages.length === 0" class="ai-empty-state">
           <div class="ai-empty-icon">
             <i class="bi bi-robot"></i>
           </div>
           <p class="ai-empty-title fw-medium mb-1">{{ $t('ai_welcome_title') || '有什么可以帮助您的？' }}</p>
           <p class="ai-empty-hint" v-html="$t('ai_welcome_msg')"></p>
           <div class="ai-quick-prompts mt-3">
             <button v-for="prompt in quickPrompts" :key="prompt" class="btn btn-sm ai-quick-prompt-btn me-2 mb-2" @click="inputText = prompt; sendMessage()">
               <i class="bi bi-lightning-charge me-1"></i>{{ prompt }}
             </button>
           </div>
        </div>

        <div v-for="(msg, idx) in currentMessages" :key="idx" class="ai-message-wrapper">
          <!-- User Message -->
          <div v-if="msg.role === 'user'" class="d-flex justify-content-end mb-3">
            <div class="ai-user-bubble">
              <div v-if="msg.files && msg.files.length" class="mb-2 d-flex flex-wrap gap-1">
                <div v-for="f in msg.files" :key="f" class="ai-file-badge">
                  <i class="bi bi-paperclip me-1"></i>{{ f }}
                </div>
              </div>
              <div class="ai-bubble-content" style="white-space: pre-wrap">{{ msg.content }}</div>
            </div>
          </div>

          <!-- Assistant Message -->
          <div v-else-if="msg.role === 'assistant'" class="d-flex justify-content-start mb-3">
            <div class="ai-assistant-bubble">
              <div class="ai-assistant-header">
                <div class="ai-assistant-avatar">
                  <i class="bi bi-stars"></i>
                </div>
                <span class="ai-assistant-name">{{ $t('ai_assistant_label') }}</span>
              </div>

              <!-- Reasoning / Thinking Block -->
              <div v-if="msg.reasoning && msg.reasoning.length > 0" class="ai-reasoning-block" :class="{ 'ai-reasoning-collapsed': !msg.showReasoning }">
                 <div class="ai-reasoning-header cursor-pointer" @click="msg.showReasoning = !msg.showReasoning">
                    <i class="bi me-1" :class="msg.showReasoning ? 'bi-chevron-down' : 'bi-chevron-right'"></i>
                    <i class="bi bi-lightbulb-fill me-1 text-warning"></i> {{ $t('ai_thought_process') }}
                    <span v-if="msg.isReasoning" class="ai-thinking-indicator"></span>
                 </div>
                 <div class="ai-reasoning-content-wrapper">
                   <div class="ai-reasoning-content" style="white-space: pre-wrap; word-break: break-all;">
                    {{ msg.reasoning }}
                   </div>
                 </div>
              </div>

              <div class="markdown-body ai-markdown" v-html="formatMarkdown(msg.reply)"></div>

              <!-- Action Plan Execution Block -->
              <div v-if="msg.actions && msg.actions.length > 0" class="ai-action-plan">
                 <div class="ai-action-plan-header">
                   <i class="bi bi-list-check me-2"></i>{{ $t('ai_action_plan') }}
                   <span class="ai-action-status-badge" :class="msg.status === 'executed' ? 'ai-status-executed' : 'ai-status-pending'">
                     {{ msg.status === 'executed' ? $t('ai_executed') : $t('ai_pending') }}
                   </span>
                 </div>
                 <ol class="ai-action-list">
                    <li v-for="(act, aIdx) in msg.actions" :key="aIdx" class="ai-action-item">
                       <span class="ai-action-tag" :class="'ai-action-' + act.type">
                         {{ act.type === 'create_product' ? $t('ai_create_product') :
                            act.type === 'create_device' ? $t('ai_create_device') :
                            act.type === 'update_mapping' ? $t('ai_update_mapping') :
                            act.type === 'update_product' ? '更新产品' :
                            act.type === 'update_device' ? '更新设备' :
                            act.type === 'delete_product' ? '删除产品' :
                            act.type === 'delete_device' ? '删除设备' :
                            $t('ai_unknown_action') }}
                       </span>

                       <template v-if="act.type === 'create_product'">
                          产品名称: <code>{{ act.payload.name }}</code> ({{ act.payload.protocol_name }})
                          <span v-if="act.payload.code" class="text-muted small ms-1">编码: {{ act.payload.code }}</span>
                       </template>
                       <template v-else-if="act.type === 'create_device'">
                          设备名称: <code>{{ act.payload.name }}</code>
                          <span v-if="act.payload.config && Object.keys(act.payload.config).length" class="text-muted small ms-1 d-block mt-1">
                            参数: {{ JSON.stringify(act.payload.config) }}
                          </span>
                       </template>
                       <template v-else-if="act.type === 'update_mapping'">
                          <span v-html="$t('ai_points_count', { count: `<strong>${act.payload.generated_points?.length || 0}</strong>` })"></span>
                       </template>
                       <template v-else-if="act.type === 'update_product'">
                          <span v-if="act.payload.config && act.payload.config.tsl">更新物模型</span>
                          <span v-else>更新产品配置</span>
                          <span class="text-muted small ms-1">目标编码: <code>{{ act.payload.code }}</code></span>
                          <div v-if="act.payload.config && act.payload.config.tsl" class="text-muted small mt-1">
                            <span v-if="act.payload.config.tsl.properties?.length">属性: {{ act.payload.config.tsl.properties.length }}个 </span>
                            <span v-if="act.payload.config.tsl.events?.length">事件: {{ act.payload.config.tsl.events.length }}个 </span>
                            <span v-if="act.payload.config.tsl.services?.length">服务: {{ act.payload.config.tsl.services.length }}个</span>
                          </div>
                       </template>
                       <template v-else-if="act.type === 'update_device' || act.type === 'delete_product' || act.type === 'delete_device'">
                          {{ $t('ai_target_code') }} <code>{{ act.payload.code }}</code>
                       </template>
                       <template v-else>
                          <span class="text-muted small d-block text-truncate" style="max-width: 200px;">
                             {{ JSON.stringify(act.payload || act.parameters || {}) }}
                          </span>
                       </template>

                       <!-- Execution Status Icon -->
                       <div v-if="act.exec_status" class="mt-1 small">
                          <span v-if="act.exec_status === 'executing'" class="text-primary"><i class="spinner-border spinner-border-sm me-1" style="width: 0.8rem; height: 0.8rem;"></i>执行中...</span>
                          <span v-else-if="act.exec_status === 'success'" class="text-success"><i class="bi bi-check-circle me-1"></i>执行成功</span>
                          <span v-else-if="act.exec_status === 'skipped'" class="text-secondary"><i class="bi bi-dash-circle me-1"></i>已合并执行</span>
                          <span v-else-if="act.exec_status === 'error'" class="text-danger"><i class="bi bi-x-circle me-1"></i>执行失败: {{ act.exec_error }}</span>
                       </div>
                    </li>
                 </ol>
                 <div class="ai-action-footer">
                    <button
                      v-if="msg.status !== 'executed'"
                      class="btn btn-sm ai-execute-btn"
                      @click="executePlan(msg)"
                      :disabled="isExecuting"
                    >
                       <span v-if="isExecuting" class="spinner-border spinner-border-sm me-1"></span>
                       <i class="bi bi-play-fill me-1" v-else></i> {{ $t('ai_execute_all') }}
                    </button>
                    <span v-else class="ai-exec-success">
                      <i class="bi bi-check-circle-fill me-1"></i>{{ $t('ai_exec_success') }}
                    </span>
                 </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Typing indicator -->
        <div v-if="isTyping" class="d-flex justify-content-start mb-3">
           <div class="ai-typing-indicator">
             <div class="ai-typing-avatar">
               <i class="bi bi-stars"></i>
             </div>
             <div class="ai-typing-dots">
               <span></span><span></span><span></span>
             </div>
           </div>
        </div>
      </div>

      <!-- Input Area -->
      <div v-if="!showHistory" class="ai-input-area">
        <!-- Attachments Preview -->
        <div v-if="attachments.length > 0" class="ai-attachments-preview">
           <span v-for="(file, idx) in attachments" :key="idx" class="ai-attachment-badge">
              <img v-if="file.type === 'image'" :src="file.content" class="ai-attachment-thumb" alt="preview"/>
              <i v-else class="bi bi-file-earmark-text"></i>
              <span class="ai-attachment-name">{{ file.name }}</span>
              <i class="bi bi-x ai-attachment-remove" @click="removeAttachment(idx)"></i>
           </span>
        </div>
        <div class="ai-input-wrapper" :class="isDarkMode ? 'ai-input-dark' : 'ai-input-light'">
          <textarea
            v-model="inputText"
            class="ai-textarea"
            rows="3"
            :placeholder="$t('ai_input_placeholder')"
            @keydown.enter.prevent="sendMessage"
            @paste="handlePaste"
          ></textarea>
          <div class="ai-input-actions">
            <div class="ai-input-tools">
              <input type="file" ref="fileInput" class="d-none" @change="handleFileUpload" accept=".csv,.xlsx,.xls,.json,.txt,.doc,.docx,.png,.jpg,.jpeg,.gif,.webp" />
              <button class="btn ai-tool-btn" @click="triggerFileUpload" :title="$t('ai_upload_attachment')" :disabled="isUploading">
                 <span v-if="isUploading" class="spinner-border spinner-border-sm"></span>
                 <i class="bi bi-paperclip" v-else></i>
              </button>
              <div v-if="providers.length > 1" class="ai-model-selector">
                <i class="bi bi-cpu"></i>
                <select v-model="selectedProvider" class="ai-model-select">
                  <option v-for="prov in providers" :key="prov.name" :value="prov.name" :disabled="!prov.has_api_key">
                    {{ prov.label }}
                  </option>
                </select>
              </div>
            </div>
            <button class="btn ai-send-btn" @click="sendMessage" :disabled="(!inputText.trim() && attachments.length === 0) || isTyping || isUploading">
              <i class="bi bi-send-fill me-1"></i> {{ $t('ai_send') }}
            </button>
          </div>
        </div>
      </div>
      <!-- 8-Direction Resize Handles -->
      <div class="resize-handle n" @mousedown.stop.prevent="startResize($event, 'n')"></div>
      <div class="resize-handle e" @mousedown.stop.prevent="startResize($event, 'e')"></div>
      <div class="resize-handle s" @mousedown.stop.prevent="startResize($event, 's')"></div>
      <div class="resize-handle w" @mousedown.stop.prevent="startResize($event, 'w')"></div>
      <div class="resize-handle ne" @mousedown.stop.prevent="startResize($event, 'ne')"></div>
      <div class="resize-handle nw" @mousedown.stop.prevent="startResize($event, 'nw')"></div>
      <div class="resize-handle se" @mousedown.stop.prevent="startResize($event, 'se')"></div>
      <div class="resize-handle sw" @mousedown.stop.prevent="startResize($event, 'sw')"></div>
    </div>
  </div>
</template>

<script>
import axios from 'axios'
import { marked } from 'marked'

export default {
  name: 'GlobalAiCopilot',
  data() {
    return {
      isOpen: false,
      isExpanded: false,
      showHistory: false,
      inputText: '',
      sessions: [], // [{ id, title, messages }]
      currentSessionId: null,
      isTyping: false,
      isExecuting: false,
      attachments: [],
      isUploading: false,
      // Position and size persistence
      position: { right: 32, bottom: 32 },
      windowSize: { width: 400, height: 600 },
      savedExpandedSize: { width: 700, height: 800 },
      // Dragging state
      isDragging: false,
      dragType: null, // 'fab' or 'window'
      startMousePos: { x: 0, y: 0 },
      startElemPos: { right: 0, bottom: 0 },
      // Resizing state
      isResizing: false,
      resizeDirection: null,
      startResizePos: { x: 0, y: 0 },
      startWindowSize: { width: 0, height: 0 },
      clickTimeout: null,
      // Promo State
      showPromo: false,
      promoMessage: '试试问我："当前网关的CPU负载怎么样？"',
      promoTimer: null,
      promoHideTimer: null,
      // Theme state for reactivity
      themeKey: Date.now(),
      // Multi-LLM provider state
      providers: [],
      selectedProvider: null,
      quickPrompts: [
        '当前网关状态如何？',
        '帮我创建产品',
        '列出离线设备'
      ]
    }
  },
  computed: {
    isDarkMode() {
      // depend on themeKey to force re-evaluation
      this.themeKey;
      const theme = localStorage.getItem('theme') || 'dark';
      return theme === 'dark' || (theme === 'system' && window.matchMedia('(prefers-color-scheme: dark)').matches);
    },
    currentMessages() {
      if (!this.currentSessionId) return [];
      const session = this.sessions.find(s => s.id === this.currentSessionId);
      return session ? session.messages : [];
    },
    dialogPositionStyle() {
      const style = {
        width: this.windowSize.width + 'px',
        height: this.windowSize.height + 'px'
      };
      
      // Calculate screen boundaries safely
      const vw = Math.max(document.documentElement.clientWidth || 0, window.innerWidth || 0);
      const vh = Math.max(document.documentElement.clientHeight || 0, window.innerHeight || 0);

      const fabRight = this.position.right;
      const fabBottom = this.position.bottom;
      const diagWidth = this.windowSize.width;
      const diagHeight = this.windowSize.height;

      // Base anchor position calculations
      let calcRight = fabRight;
      let calcBottom = fabBottom + 80; // default 80px offset from bottom

      // Boundary clamp for X-axis
      // Left edge off-screen check:
      if (vw - calcRight - diagWidth < 10) {
          calcRight = vw - diagWidth - 10;
      }
      // Right edge off-screen check:
      if (calcRight < 10) {
          calcRight = 10;
      }

      // Boundary clamp for Y-axis
      // Top edge off-screen check:
      if (vh - calcBottom - diagHeight < 10) {
          // If it overflows top, we try to drop it below the FAB instead if there is room.
          if (fabBottom > diagHeight + 20) {
             calcBottom = fabBottom - diagHeight - 20;
          } else {
             // Just hard cap it to screen top
             calcBottom = vh - diagHeight - 10;
          }
      }
      // Bottom edge off-screen check:
      if (calcBottom < 10) {
          calcBottom = 10;
      }

      style.right = calcRight + 'px';
      style.left = 'auto';
      style.bottom = calcBottom + 'px';
      style.top = 'auto';

      return style;
    }
  },
  mounted() {
    this.loadHistory();
    this.loadLayout();
    this.loadProviders();
    this.initPromoSchedule();
    window.addEventListener('mousemove', this.onMouseMove);
    window.addEventListener('mouseup', this.onMouseUp);
    window.addEventListener('noyo-open-copilot', this.handleOpenEvent);
    window.addEventListener('noyo-theme-changed', this.handleThemeChange);
    window.addEventListener('noyo-analyze-log', this.handleLogAnalysis);
    
    // Listen for system theme changes
    this.systemThemeQuery = window.matchMedia('(prefers-color-scheme: dark)');
    if (this.systemThemeQuery.addEventListener) {
      this.systemThemeQuery.addEventListener('change', this.handleThemeChange);
    } else {
      this.systemThemeQuery.addListener(this.handleThemeChange); // Fallback for older browsers
    }

    this.unwatchTheme = this.$watch('themeKey', () => {
      this.$forceUpdate();
    });
  },
  beforeUnmount() {
    window.removeEventListener('mousemove', this.onMouseMove);
    window.removeEventListener('mouseup', this.onMouseUp);
    window.removeEventListener('noyo-open-copilot', this.handleOpenEvent);
    window.removeEventListener('noyo-theme-changed', this.handleThemeChange);
    window.removeEventListener('noyo-analyze-log', this.handleLogAnalysis);
    
    if (this.systemThemeQuery) {
      if (this.systemThemeQuery.removeEventListener) {
        this.systemThemeQuery.removeEventListener('change', this.handleThemeChange);
      } else {
        this.systemThemeQuery.removeListener(this.handleThemeChange);
      }
    }

    if (this.unwatchTheme) this.unwatchTheme();
    if (this.promoTimer) clearTimeout(this.promoTimer);
    if (this.promoHideTimer) clearTimeout(this.promoHideTimer);
  },
  watch: {
    sessions: {
      deep: true,
      handler(newVal) {
        try {
           localStorage.setItem('noyo_copilot_sessions', JSON.stringify(newVal));
        } catch (e) {
           console.warn('Failed to save AI sessions to localStorage (quota exceeded?)', e);
        }
      }
    },
    currentSessionId(newVal) {
      if (newVal) {
        localStorage.setItem('noyo_copilot_active_session', newVal.toString());
      }
    },
    selectedProvider(newVal) {
      if (newVal) {
        localStorage.setItem('noyo_copilot_provider', newVal);
      }
    },
    isOpen(newVal) {
      if (newVal) {
        this.loadProviders();
        this.$nextTick(() => {
          setTimeout(() => this.scrollToBottom(), 100);
        });
      }
    }
  },
  methods: {
    handleOpenEvent() {
      if (!this.isOpen) {
        this.handleClick();
      }
    },
    handleLogAnalysis(e) {
      if (!this.isOpen) {
        this.toggleChat();
      }
      const text = e.detail.text;
      if (!text) return;
      
      this.createNewSession();
      
      const promptText = `请帮我分析以下日志内容：\n\n\`\`\`\n${text}\n\`\`\``;
      
      // Add message with special flag
      this.addMessage({ role: 'user', content: promptText, _rawContent: promptText, files: [], _isLogAnalysis: true });
      this.inputText = '';
      this.attachments = [];
      this.isTyping = true;
      this.scrollToBottom();
      
      // Proceed with API call
      this.triggerAiApiCall();
    },
    handleThemeChange(e) {
      this.themeKey = Date.now();
    },
    loadHistory() {
      try {
        // Migrate old history if present
        const oldHistory = localStorage.getItem('noyo_copilot_history');
        if (oldHistory) {
            this.sessions.push({
                id: Date.now() - 10000,
                title: this.$t('ai_old_history_title'),
                messages: JSON.parse(oldHistory)
            });
            localStorage.removeItem('noyo_copilot_history');
        }

        const stored = localStorage.getItem('noyo_copilot_sessions');
        if (stored) {
          const parsed = JSON.parse(stored);
          if (Array.isArray(parsed) && parsed.length > 0) {
             this.sessions.push(...parsed.filter(s => Array.isArray(s.messages)));
          }
        }
        
        if (this.sessions.length === 0) {
          this.createNewSession();
        } else {
          const storedActive = localStorage.getItem('noyo_copilot_active_session');
          if (storedActive && this.sessions.some(s => s.id === Number(storedActive))) {
            this.currentSessionId = Number(storedActive);
          } else {
            this.currentSessionId = this.sessions[0].id;
          }
          setTimeout(() => this.scrollToBottom(), 100);
        }
      } catch (e) {
        console.error("Failed to load AI chat history", e);
        if (this.sessions.length === 0) this.createNewSession();
      }
    },
    createNewSession() {
      const newSession = {
        id: Date.now(),
        title: this.$t('ai_new_session_title') + ' ' + new Date().toLocaleTimeString(),
        messages: []
      };
      this.sessions.unshift(newSession);
      this.currentSessionId = newSession.id;
      this.showHistory = false;
      this.scrollToBottom();
    },
    selectSession(id) {
      this.currentSessionId = id;
      this.showHistory = false;
      this.scrollToBottom();
    },
    deleteSession(id) {
        if (confirm(this.$t('ai_confirm_del_session'))) {
            const index = this.sessions.findIndex(s => s.id === id);
            if (index !== -1) {
                this.sessions.splice(index, 1);
            }
            if (this.sessions.length === 0) {
                this.createNewSession();
            } else if (this.currentSessionId === id) {
                this.currentSessionId = this.sessions[0].id;
            }
        }
    },
    addMessage(msg) {
        const session = this.sessions.find(s => s.id === this.currentSessionId);
        if (session) {
            session.messages.push(msg);
            if (session.messages.length === 1 && msg.role === 'user') {
                session.title = msg.content.substring(0, 15) + (msg.content.length > 15 ? '...' : '');
            }
        }
    },
    toggleChat() {
      this.isOpen = !this.isOpen
    },
    // Layout persistence
    loadLayout() {
      const pos = localStorage.getItem('noyo_copilot_pos');
      if (pos) this.position = JSON.parse(pos);
      const size = localStorage.getItem('noyo_copilot_size');
      if (size) this.windowSize = JSON.parse(size);
      const expandedSize = localStorage.getItem('noyo_copilot_expanded_size');
      if (expandedSize) this.savedExpandedSize = JSON.parse(expandedSize);
      const expanded = localStorage.getItem('noyo_copilot_is_expanded');
      if (expanded) this.isExpanded = JSON.parse(expanded);
    },
    saveLayout() {
      localStorage.setItem('noyo_copilot_pos', JSON.stringify(this.position));
      localStorage.setItem('noyo_copilot_size', JSON.stringify(this.windowSize));
      localStorage.setItem('noyo_copilot_expanded_size', JSON.stringify(this.savedExpandedSize));
      localStorage.setItem('noyo_copilot_is_expanded', JSON.stringify(this.isExpanded));
    },
    async loadProviders() {
      try {
        const res = await axios.get('/api/extension/ai/providers');
        if (res.data && res.data.code === 0 && Array.isArray(res.data.data)) {
          this.providers = res.data.data;
          const saved = localStorage.getItem('noyo_copilot_provider');
          if (saved && this.providers.some(p => p.name === saved)) {
            this.selectedProvider = saved;
          } else if (this.providers.length > 0) {
            this.selectedProvider = this.providers[0].name;
          }
        }
      } catch (e) {
        console.warn('Failed to load AI providers', e);
      }
    },
    // Interaction Handlers
    triggerFileUpload() {
       this.$refs.fileInput.click();
    },
    async handleFileUpload(event) {
       const file = event.target.files[0];
       if (!file) return;
       // Reset input
       event.target.value = '';
       
       if (file.size > 5 * 1024 * 1024) {
           alert("文件太大，不能超过5MB哦");
           return;
       }

       this.isUploading = true;
       const formData = new FormData();
       formData.append('file', file);

       try {
           const res = await axios.post('/api/extension/ai/parse_file', formData, {
               headers: { 'Content-Type': 'multipart/form-data' }
           });
           if (res.data && res.data.code === 0) {
               this.attachments.push({
                   name: res.data.filename || file.name,
                   content: res.data.data || '',
                   type: res.data.file_type || 'text'
               });
           } else {
               alert("解析文件失败：" + (res.data ? res.data.message : '未知错误'));
           }
       } catch (err) {
           console.error(err);
           alert("上传错误: " + err.message);
       } finally {
           this.isUploading = false;
       }
    },
    async handlePaste(event) {
        // Check if clipboard contains any items
        if (!event.clipboardData || !event.clipboardData.items) return;

        const items = event.clipboardData.items;
        let imageFile = null;

        // Loop through clipboard items to find an image
        for (let i = 0; i < items.length; i++) {
            if (items[i].type.indexOf('image') !== -1) {
                imageFile = items[i].getAsFile();
                break;
            }
        }

        if (imageFile) {
            // Prevent default paste behavior (which would just paste text if any)
            event.preventDefault();
            
            // Re-use the existing file upload logic by faking an event target
            const fakeEvent = {
                target: {
                    files: [imageFile],
                    value: ''
                }
            };
            await this.handleFileUpload(fakeEvent);
        }
    },
    removeAttachment(idx) {
       this.attachments.splice(idx, 1);
    },
    handleClick() {
      // Small timeout to differentiate between drag and click
      if (this.isDragging) return;
      this.closePromo(); // Hide promo if open
      this.toggleChat();
    },
    // Promo Logic
    initPromoSchedule() {
       // Show 5 seconds after page load unconditionally
       this.promoTimer = setTimeout(() => {
           this.triggerPromo();
       }, 5000);
       
       // Then repeat every 5 minutes (for debugging & long sessions)
       setInterval(() => {
           this.triggerPromo();
       }, 5 * 60 * 1000);
    },
    triggerPromo() {
       if (!this.isOpen) {
          const promos = [
            "试试问我：“网关当前的CPU负载怎么样？”",
            "我可以帮您快速创建产品和设备哦！",
            "试试问我：“列出所有离线的设备。”",
            "有问题随时问我，为您解放双手！"
          ];
          this.promoMessage = promos[Math.floor(Math.random() * promos.length)];
          this.showPromo = true;
          localStorage.setItem('noyo_copilot_last_promo', Date.now().toString());

          // Auto hide after 8 seconds
          if (this.promoHideTimer) clearTimeout(this.promoHideTimer);
          this.promoHideTimer = setTimeout(() => {
             this.showPromo = false;
          }, 8000);
       }
    },
    closePromo() {
       this.showPromo = false;
       if (this.promoHideTimer) clearTimeout(this.promoHideTimer);
    },
    startDrag(e, type) {
      this.isDragging = false; // Reset
      this.dragType = type;
      this.startMousePos = { x: e.clientX, y: e.clientY };
      this.startElemPos = { ...this.position };
      
      // We don't set isDragging true immediately to allow for clicks
      document.body.style.userSelect = 'none';
    },
    startResize(e, dir) {
      e.preventDefault();
      e.stopPropagation();
      this.isResizing = true;
      this.resizeDirection = dir;
      this.startResizePos = { x: e.clientX, y: e.clientY };
      this.startWindowSize = { ...this.windowSize };
      this.startElemPos = { ...this.position };
      document.body.style.userSelect = 'none';
      
      // Prevent dialog style snapping issue by temporarily freezing bounding swap
      const s = this.dialogPositionStyle; // current computed style
      if (s.left !== 'auto') {
         // It was snapped left. Convert left coordinate back to right anchor math
         // Actually, if it's flipped, the math gets extremely complex. 
         // For now we assume standard right-bottom anchoring for resize math.
      }
    },
    onMouseMove(e) {
      if (this.dragType) {
        const dx = e.clientX - this.startMousePos.x;
        const dy = e.clientY - this.startMousePos.y;
        
        if (Math.abs(dx) > 5 || Math.abs(dy) > 5) {
          this.isDragging = true;
        }
        
        if (this.isDragging) {
          this.position.right = Math.max(10, this.startElemPos.right - dx);
          this.position.bottom = Math.max(10, this.startElemPos.bottom - dy);
        }
      }
      
      if (this.isResizing) {
        const dx = e.clientX - this.startResizePos.x;
        const dy = e.clientY - this.startResizePos.y;
        const dir = this.resizeDirection;
        const minW = 300, minH = 400;

        let newW = this.startWindowSize.width;
        let newH = this.startWindowSize.height;
        let newR = this.startElemPos.right;
        let newB = this.startElemPos.bottom;

        // West edge (Left)
        if (dir.includes('w')) {
           newW = Math.max(minW, this.startWindowSize.width - dx);
        }
        // East edge (Right)
        if (dir.includes('e')) {
           newW = Math.max(minW, this.startWindowSize.width + dx);
           if (newW > minW) newR = this.startElemPos.right - dx;
        }
        // North edge (Top)
        if (dir.includes('n')) {
           newH = Math.max(minH, this.startWindowSize.height - dy);
        }
        // South edge (Bottom)
        if (dir.includes('s')) {
           newH = Math.max(minH, this.startWindowSize.height + dy);
           if (newH > minH) newB = this.startElemPos.bottom - dy;
        }

        this.windowSize.width = newW;
        this.windowSize.height = newH;
        this.position.right = newR;
        this.position.bottom = newB;
      }
    },
    onMouseUp() {
      if (this.isResizing) {
        this.isExpanded = true;
        this.savedExpandedSize = { ...this.windowSize };
      }
      if (this.isDragging || this.isResizing) {
        this.saveLayout();
      }
      this.dragType = null;
      // We don't reset isDragging immediately so the click handler knows it was a drag
      setTimeout(() => { this.isDragging = false; }, 50);
      this.isResizing = false;
      document.body.style.userSelect = '';
    },
    toggleExpand() {
      this.isExpanded = !this.isExpanded;
      if (this.isExpanded) {
        // Expand to previously saved large size or default 700x800
        this.windowSize = { ...this.savedExpandedSize };
      } else {
        // Collapse to default small size
        this.windowSize = { width: 400, height: 600 };
      }
      this.saveLayout();
    },
    formatMarkdown(text) {
       if (!text) return '';
       try {
         return marked.parse(text);
       } catch (e) {
         console.error('Markdown parsing error', e);
         return text.replace(/\n/g, '<br/>');
       }
    },
    async sendMessage() {
      const text = this.inputText.trim();
      if (!text && this.attachments.length === 0) return;

      let rawContent = text;
      const fileNames = [];
      if (this.attachments.length > 0) {
          const hasImages = this.attachments.some(a => a.type === 'image');
          if (hasImages) {
              // Construct multi-modal array payload
              rawContent = [];
              if (text) {
                 rawContent.push({ type: "text", text: text });
              }
              
              this.attachments.forEach(a => {
                  fileNames.push(a.name);
                  if (a.type === 'image') {
                      rawContent.push({ type: "image_url", image_url: { url: a.content } });
                  } else {
                      rawContent.push({ type: "text", text: `\n\n--- 文件: ${a.name} ---\n${a.content}\n--- 文件结束 ---` });
                  }
              });
          } else {
              // Only text files attached
              const contextBlocks = this.attachments.map(a => {
                 fileNames.push(a.name);
                 return `\n\n--- 文件: ${a.name} ---\n${a.content}\n--- 文件结束 ---`;
              });
              rawContent = text + contextBlocks.join('');
          }
      }

      // Add user message to UI
      this.addMessage({ role: 'user', content: text, _rawContent: rawContent, files: fileNames });
      this.inputText = '';
      this.attachments = [];
      this.isTyping = true;
      this.scrollToBottom();

      this.triggerAiApiCall();
    },
    async triggerAiApiCall() {
      try {
        // Check if the last user message was a log analysis
        const lastUserMsg = this.currentMessages[this.currentMessages.length - 1];
        const isLogAnalysis = lastUserMsg && lastUserMsg._isLogAnalysis;
        const endpoint = isLogAnalysis ? '/api/extension/ai/analyze_log' : '/api/extension/ai/chat';

        // Construct API payload matching backend Expectation for chat history
        const apiMessages = this.currentMessages.map(m => {
           if (m.role === 'user') return { role: 'user', content: m._rawContent || m.content };
           // For assistant, we send back its reply so it has context
           if (m.role === 'assistant') return { role: 'assistant', content: m.reply };
           return null;
        }).filter(Boolean);

        const session = this.sessions.find(s => s.id === this.currentSessionId);
        if (!session) throw new Error("No active session found");
        
        const currentMsgIndex = session.messages.length;
        session.messages.push({
           role: 'assistant',
           reply: '',
           reasoning: '',
           showReasoning: true,
           isReasoning: true,
           actions: [],
           status: 'pending'
        });
        
        const assistantMsg = session.messages[currentMsgIndex];
        
        const response = await fetch(endpoint, {
           method: 'POST',
           headers: { 'Content-Type': 'application/json' },
           body: JSON.stringify({ messages: apiMessages, provider_name: this.selectedProvider || '' })
        });
        
        // 移除 !response.ok 的绝对抛出，因为后端会在错误时也尝试返回具有 {"error": ...} 的 SSE 数据流
        const reader = response.body.getReader();
        const decoder = new TextDecoder("utf-8");
        let done = false;
        let aiFullContent = ""; // Collect raw JSON string
        let buffer = ""; // For partial stream chunks

        this.isTyping = false; // We have started receiving, no need for generic spinner

        while (!done) {
            const { value, done: readerDone } = await reader.read();
            done = readerDone;
            if (value) {
                buffer += decoder.decode(value, { stream: true });
                const lines = buffer.split("\n");
                // The last element is a partial line (or empty string if it ended with \n)
                buffer = lines.pop(); 
                
                for (const line of lines) {
                    const trimmed = line.trim();
                    if (trimmed.startsWith("data: ") && trimmed !== "data: [DONE]") {
                        const dataStr = trimmed.substring(6);
                        try {
                           const data = JSON.parse(dataStr);
                           
                           if (data.error) {
                               assistantMsg.reply += `\n\n**系统提示**：${data.error}`;
                               assistantMsg.isReasoning = false;
                               this.scrollToBottom();
                               done = true;
                               break;
                           }
                           
                           if (data.choices && data.choices.length > 0) {
                              const delta = data.choices[0].delta;
                              
                              if (delta.reasoning_content) {
                                  assistantMsg.reasoning += delta.reasoning_content;
                                  assistantMsg.isReasoning = true;
                                  this.scrollToBottom();
                              } else if (delta.content) {
                                  assistantMsg.isReasoning = false;
                                  // Auto-collapse reasoning once actual response starts
                                  if (assistantMsg.reasoning && assistantMsg.showReasoning) {
                                     assistantMsg.showReasoning = false;
                                  }
                                  aiFullContent += delta.content;
                                  // Display what we have so far
                                  assistantMsg.reply = aiFullContent; 
                                  this.scrollToBottom();
                              }
                           }
                        } catch (e) {
                           // Ignore incomplete JSON stream chunks or non-JSON data
                        }
                    }
                }
            }
        }
        
        assistantMsg.isReasoning = false;

        // 如果在流解析中已经处理了 error 消息，reply 已被设置，不要覆盖
        if (assistantMsg.reply && aiFullContent === '') {
           // 已经从 SSE error 包中设置了 reply，直接跳过 JSON 解析
        } else {
          // At end of stream, parse the full accumulated JSON string 
          // We do this to extract the structured `reply` and `actions` arrays.
          let finalTextRaw = aiFullContent.trim();
          // Remove markdown wrappers if any
          if (finalTextRaw.startsWith("```json")) finalTextRaw = finalTextRaw.substring(7);
          else if (finalTextRaw.startsWith("```")) finalTextRaw = finalTextRaw.substring(3);
          if (finalTextRaw.endsWith("```")) finalTextRaw = finalTextRaw.substring(0, finalTextRaw.length - 3);
          finalTextRaw = finalTextRaw.trim();

          try {
             const parsedJson = JSON.parse(finalTextRaw);
             
             if (parsedJson.error) {
                 assistantMsg.reply = `\n\n**系统提示**：${parsedJson.error}`;
                 assistantMsg.actions = [];
                 return;
             }

             assistantMsg.reply = parsedJson.reply || "";
             assistantMsg.actions = parsedJson.actions || [];
          } catch (e) {
             // If LLM failed to output JSON, treat everything as reply
             assistantMsg.reply = aiFullContent;
             console.error("Failed to parse final AI payload to JSON", e);
          }
        }

      } catch (err) {
        console.error("AI Stream caught error:", err);
        // Replace the pending message with the error
        const session = this.sessions.find(s => s.id === this.currentSessionId);
        if (session && session.messages.length > 0) {
            const lastMsg = session.messages[session.messages.length - 1];
            if (lastMsg.role === 'assistant' && lastMsg.status === 'pending') {
               lastMsg.reply = `**系统错误**：此模型无法处理您的请求。原因：${err.message}`;
               lastMsg.isReasoning = false;
               lastMsg.showReasoning = false;
               lastMsg.status = 'error';
            } else {
               this.addMessage({ role: 'assistant', reply: `❌ ${err.message}`, actions: [] });
            }
        } else {
            this.addMessage({ role: 'assistant', reply: `❌ ${err.message}`, actions: [] });
        }
      } finally {
        this.isTyping = false;
        this.saveLayout();
        this.scrollToBottom();
      }
    },
    scrollToBottom() {
      this.$nextTick(() => {
        const container = this.$refs.messagesContainer;
        if (container) {
          container.scrollTop = container.scrollHeight;
        }
      });
    },
    // The Action Executor Core Logic
    async executePlan(msg) {
       this.isExecuting = true;
       try {
          // Fetch existing products to avoid duplicating names and to resolve names to codes
          const prodRes = await axios.get('/api/products');
          const existingProducts = (prodRes.data && prodRes.data.data) ? prodRes.data.data : [];

          let globalProductCode = ""; // Track product code if we create one, so the device can link to it
          const aiCodeMap = {}; // Map AI-generated codes to actual codes

          let currentActionIndex = -1;

          for (let i = 0; i < msg.actions.length; i++) {
             currentActionIndex = i;
             const action = msg.actions[i];
             action.exec_status = 'executing';

             if (action.type === 'create_product') {
                const targetName = action.payload.name || 'AI生成组件';
                const aiGeneratedCode = action.payload.code;
                const existing = existingProducts.find(p => p.name === targetName);
                const nextAction = (i + 1 < msg.actions.length) ? msg.actions[i+1] : null;

                if (existing) {
                    globalProductCode = existing.code;
                    if (aiGeneratedCode) {
                        aiCodeMap[aiGeneratedCode] = existing.code;
                    }
                    action.executed_inline = true; // Mark as skipped/handled
                    action.exec_status = 'skipped';
                    
                    // If the next action is update_mapping, let it handle the existing product
                    if (nextAction && nextAction.type === 'update_mapping' && nextAction.payload && nextAction.payload.generated_points) {
                        if (!nextAction.payload.product_code && !nextAction.payload.device_code) {
                            nextAction.payload.product_code = existing.code;
                        }
                    }
                    continue; // Skip creation since it exists
                }

                const payload = {
                  code: 'P' + Date.now().toString() + Math.floor(Math.random()*1000), // Generator standard code
                  name: targetName,
                  protocol_name: action.payload.protocol_name || '',
                  config: '{}' // Default empty config
                };
                globalProductCode = payload.code;
                if (aiGeneratedCode) {
                    aiCodeMap[aiGeneratedCode] = payload.code;
                }

                // Look ahead for update_mapping
                if (nextAction && nextAction.type === 'update_mapping' && nextAction.payload && nextAction.payload.generated_points) {
                   const configObj = {};
                   configObj.points = nextAction.payload.generated_points.map(pt => ({
                       ...pt,
                       type: pt.type || 'property', // Default to property if not provided
                       is_property: pt.is_property !== false,
                       interval: 1000,
                       byte_order: pt.byte_order || "ABCD",
                       slave_id: 1
                   }));

                   // Automatically generate abstract TSL for the product so UI recognizes it
                   configObj.tsl = { properties: [], events: [], services: [] };
                   
                   const parseIOData = (arr) => {
                       if (!Array.isArray(arr)) return [];
                       return arr.map(a => ({
                           identifier: a.name || 'param',
                           name: a.display_name || a.name || 'Parameter',
                           dataType: a.data_type || { type: 'int' }
                       }));
                   };

                   nextAction.payload.generated_points.forEach(pt => {
                       const tType = pt.type || 'property';
                       if (tType === 'property') {
                           configObj.tsl.properties.push({
                               identifier: pt.name,
                               name: pt.display_name || pt.name,
                               dataType: { type: pt.data_type && pt.data_type.includes('float') ? 'float' : 'int' },
                               accessMode: pt.enable_write ? "rw" : "r"
                           });
                       } else if (tType === 'event') {
                           configObj.tsl.events.push({ 
                               identifier: pt.name, 
                               name: pt.display_name || pt.name,
                               type: pt.event_type || 'info',
                               outputData: parseIOData(pt.output_data)
                           });
                       } else if (tType === 'service') {
                           configObj.tsl.services.push({ 
                               identifier: pt.name, 
                               name: pt.display_name || pt.name,
                               callType: pt.call_type || 'async',
                               inputData: parseIOData(pt.input_data),
                               outputData: parseIOData(pt.output_data)
                           });
                       }
                   });

                   payload.config = JSON.stringify(configObj);
                   nextAction.executed_inline = true; 
                   nextAction.exec_status = 'skipped';
                }

                const res = await axios.post('/api/products', payload);
                if (res.data.code !== 0) throw new Error("创建产品失败: " + res.data.message);
                
             } 
             else if (action.type === 'create_device') {
                let pCode = action.payload.product_code;
                if (pCode && (pCode.includes('隐式关联') || pCode.includes('上一步'))) {
                    pCode = ""; // Clear it so it falls back to globalProductCode
                } else if (pCode && aiCodeMap[pCode]) {
                    pCode = aiCodeMap[pCode];
                } else if (pCode && globalProductCode && !existingProducts.find(p => p.code === pCode)) {
                    // Fallback: If AI hallucinated a code not in DB, but we just created a product, assume it meant that one
                    pCode = globalProductCode;
                }
                // If AI hallucinated a name as the product_code, resolve it
                if (pCode) {
                    const match = existingProducts.find(p => p.name === pCode);
                    if (match) pCode = match.code;
                }
                
                let parentCode = action.payload.parent_code || '';
                if (parentCode && aiCodeMap[parentCode]) {
                    parentCode = aiCodeMap[parentCode];
                }
                
                const payload = {
                  code: 'D' + Date.now().toString() + Math.floor(Math.random()*1000),
                  name: action.payload.name || 'AI生成设备',
                  product_code: pCode || globalProductCode,
                  parent_code: parentCode,
                  enabled: action.payload.enabled !== false,
                  config: JSON.stringify(action.payload.config || {})
                };

                if (action.payload.code) {
                    aiCodeMap[action.payload.code] = payload.code;
                }
                
                const res = await axios.post('/api/devices', payload);
                if (res.data.code !== 0) throw new Error("创建设备失败: " + res.data.message);
             }
             else if (action.type === 'update_mapping') {
                if (action.executed_inline) {
                    action.exec_status = 'skipped';
                    continue; // Skip if done during device creation
                }

                let targetCode = action.payload.product_code || globalProductCode;
                if (aiCodeMap[targetCode]) {
                    targetCode = aiCodeMap[targetCode];
                } else if (globalProductCode && !existingProducts.find(p => p.code === targetCode)) {
                    targetCode = globalProductCode;
                }
                let targetType = 'products';
                
                if (action.payload.device_code && !targetCode) {
                   throw new Error("诺优 的物模型(点位)必须配置在产品(Product)上，不能直接配置在具体设备上。由于未找到可用的产品编码，更新终止。请确保明确指定 product_code。");
                }

                if (!targetCode) {
                   throw new Error("单独更新点位未能找到目标产品编码，请明确指定产品编码或紧跟在产品创建动作之后。");
                }

                const getRes = await axios.get(`/api/${targetType}/${targetCode}`);
                if (getRes.data.code !== 0) throw new Error(`获取目标产品以便更新物模型点位失败: ${getRes.data.message}`);

                const targetObj = getRes.data.data;
                let configObj = {};
                if (targetObj.config) {
                   try { configObj = JSON.parse(targetObj.config); } catch (e) {}
                }

                configObj.points = action.payload.generated_points.map(pt => ({
                    ...pt,
                    type: pt.type || 'property', // Default to property if not provided
                    is_property: pt.is_property !== false,
                    interval: 1000,
                    byte_order: pt.byte_order || "ABCD",
                    slave_id: 1
                }));

                // Ensure TSL exists and merge generated points into abstract TSL
                if (!configObj.tsl) {
                    configObj.tsl = { properties: [], events: [], services: [] };
                }

                const parseIOData = (arr) => {
                    if (!Array.isArray(arr)) return [];
                    return arr.map(a => ({
                        identifier: a.name || 'param',
                        name: a.display_name || a.name || 'Parameter',
                        dataType: a.data_type || { type: 'int' }
                    }));
                };

                action.payload.generated_points.forEach(pt => {
                    const tType = pt.type || 'property';
                    if (tType === 'property') {
                        const existing = configObj.tsl.properties.find(p => p.identifier === pt.name);
                        if (!existing) {
                            configObj.tsl.properties.push({
                                identifier: pt.name,
                                name: pt.display_name || pt.name,
                                dataType: { type: pt.data_type && pt.data_type.includes('float') ? 'float' : 'int' },
                                accessMode: pt.enable_write ? "rw" : "r"
                            });
                        } else {
                            // Update existing property
                            existing.name = pt.display_name || pt.name;
                            existing.dataType = { type: pt.data_type && pt.data_type.includes('float') ? 'float' : 'int' };
                            existing.accessMode = pt.enable_write ? "rw" : "r";
                        }
                    } else if (tType === 'event') {
                        const existing = configObj.tsl.events.find(e => e.identifier === pt.name);
                        if (!existing) {
                            configObj.tsl.events.push({ 
                                identifier: pt.name, 
                                name: pt.display_name || pt.name,
                                type: pt.event_type || 'info',
                                outputData: parseIOData(pt.output_data) 
                            });
                        } else {
                            // Update existing event
                            existing.name = pt.display_name || pt.name;
                            existing.type = pt.event_type || 'info';
                            existing.outputData = parseIOData(pt.output_data);
                        }
                    } else if (tType === 'service') {
                        const existing = configObj.tsl.services.find(s => s.identifier === pt.name);
                        if (!existing) {
                            configObj.tsl.services.push({ 
                                identifier: pt.name, 
                                name: pt.display_name || pt.name,
                                callType: pt.call_type || 'async',
                                inputData: parseIOData(pt.input_data),
                                outputData: parseIOData(pt.output_data) 
                            });
                        } else {
                            // Update existing service
                            existing.name = pt.display_name || pt.name;
                            existing.callType = pt.call_type || 'async';
                            existing.inputData = parseIOData(pt.input_data);
                            existing.outputData = parseIOData(pt.output_data);
                        }
                    }
                });

                targetObj.config = JSON.stringify(configObj);
                const putRes = await axios.put(`/api/${targetType}/${targetCode}`, targetObj);
                if (putRes.data.code !== 0) throw new Error(`更新点位保存失败: ${putRes.data.message}`);
             }
             else if (action.type === 'update_product') {
                let targetCode = action.payload.code;
                if (!targetCode || targetCode.includes('隐式关联') || targetCode.includes('上一步')) {
                    targetCode = globalProductCode;
                } else if (aiCodeMap[targetCode]) {
                    targetCode = aiCodeMap[targetCode];
                } else if (globalProductCode && !existingProducts.find(p => p.code === targetCode)) {
                    // Fallback: If AI hallucinated a code not in DB, but we just created a product, assume it meant that one
                    targetCode = globalProductCode;
                }
                if (!targetCode) throw new Error("更新产品缺少目标产品编码");

                // 获取已有的产品信息，避免覆盖丢失原有的 Name 和 ProtocolName
                const getRes = await axios.get('/api/products/' + targetCode);
                if (getRes.data.code !== 0) throw new Error("获取产品信息失败: " + getRes.data.message);
                const existingObj = getRes.data.data;

                const payload = { ...existingObj, ...action.payload };
                payload.code = targetCode; // Ensure payload code matches the real code
                
                // 深度合并 config
                let configObj = {};
                try { configObj = JSON.parse(existingObj.config || '{}'); } catch(e) {}
                
                if (action.payload.config) {
                    let newConfig = action.payload.config;
                    if (typeof newConfig === 'string') {
                        try { newConfig = JSON.parse(newConfig); } catch(e) {}
                    }
                    if (typeof newConfig === 'object') {
                        configObj = { ...configObj, ...newConfig };
                    }
                }
                payload.config = JSON.stringify(configObj);
                
                const res = await axios.put('/api/products/' + targetCode, payload);
                if (res.data.code !== 0) throw new Error("更新产品失败: " + res.data.message);
             }
             else if (action.type === 'update_device') {
                let targetCode = action.payload.code;
                if (aiCodeMap[targetCode]) targetCode = aiCodeMap[targetCode];

                // 获取已有的设备信息，避免覆盖丢失原有的字段
                const getRes = await axios.get('/api/devices/' + targetCode);
                if (getRes.data.code !== 0) throw new Error("获取设备信息失败: " + getRes.data.message);
                const existingObj = getRes.data.data;

                const payload = { ...existingObj, ...action.payload };
                payload.code = targetCode; // Ensure payload code matches the real code
                if (payload.parent_code && aiCodeMap[payload.parent_code]) {
                    payload.parent_code = aiCodeMap[payload.parent_code];
                }
                if (payload.product_code && aiCodeMap[payload.product_code]) {
                    payload.product_code = aiCodeMap[payload.product_code];
                }
                
                // 深度合并 config
                let configObj = {};
                try { configObj = JSON.parse(existingObj.config || '{}'); } catch(e) {}
                
                if (action.payload.config) {
                    let newConfig = action.payload.config;
                    if (typeof newConfig === 'string') {
                        try { newConfig = JSON.parse(newConfig); } catch(e) {}
                    }
                    if (typeof newConfig === 'object') {
                        configObj = { ...configObj, ...newConfig };
                    }
                }
                payload.config = JSON.stringify(configObj);
                
                const res = await axios.put('/api/devices/' + targetCode, payload);
                if (res.data.code !== 0) throw new Error("更新设备失败: " + res.data.message);
             }
             else if (action.type === 'delete_product') {
                let targetCode = action.payload.code;
                if (aiCodeMap[targetCode]) targetCode = aiCodeMap[targetCode];
                const res = await axios.delete('/api/products/' + targetCode);
                if (res.data.code !== 0) throw new Error("删除产品失败: " + res.data.message);
             }
             else if (action.type === 'delete_device') {
                let targetCode = action.payload.code;
                if (aiCodeMap[targetCode]) targetCode = aiCodeMap[targetCode];
                const res = await axios.delete('/api/devices/' + targetCode);
                if (res.data.code !== 0) throw new Error("删除设备失败: " + res.data.message);
             }
             
             if (action.exec_status === 'executing') {
                 action.exec_status = 'success';
             }
          }

          msg.status = 'executed';
          window.dispatchEvent(new CustomEvent('noyo-data-updated'));
          alert(this.$t('ai_exec_success'));

       } catch (err) {
          if (currentActionIndex >= 0 && currentActionIndex < msg.actions.length) {
              msg.actions[currentActionIndex].exec_status = 'error';
              msg.actions[currentActionIndex].exec_error = err.message;
          }
          alert(`${this.$t('ai_exec_interrupted')}: ${err.message}`);
       } finally {
          this.isExecuting = false;
       }
    }
  }
}
</script>

<style scoped>
/* ===================== MODERN AI CHAT UI STYLES ===================== */

.global-ai-copilot {
  position: fixed;
  z-index: 9999;
}

.ai-fab-container {
  position: fixed;
  z-index: 10000;
  transition: transform 0.2s ease;
  display: flex;
  flex-direction: column;
  align-items: center;
}

.ai-fab {
  width: 56px;
  height: 56px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  border: none;
  background: linear-gradient(135deg, #6366f1 0%, #8b5cf6 50%, #a855f7 100%);
  color: #fff;
  box-shadow: 0 4px 20px rgba(99, 102, 241, 0.4), 0 0 30px rgba(139, 92, 246, 0.2);
  transition: transform 0.2s, box-shadow 0.2s;
  cursor: pointer;
  animation: fab-float 3s ease-in-out infinite;
}

.ai-fab:hover {
  transform: scale(1.08);
  box-shadow: 0 6px 28px rgba(99, 102, 241, 0.5), 0 0 40px rgba(139, 92, 246, 0.3);
  animation-play-state: paused;
}

.ai-fab:active {
  transform: scale(0.95);
  animation: none;
}

@keyframes fab-float {
  0%, 100% { transform: translateY(0); }
  50% { transform: translateY(-6px); }
}

.icon-pulse {
  animation: pulse-glow 2s ease-in-out infinite;
}

@keyframes pulse-glow {
  0%, 100% { opacity: 1; filter: drop-shadow(0 0 2px rgba(255, 255, 255, 0.5)); }
  50% { opacity: 0.85; filter: drop-shadow(0 0 8px rgba(255, 255, 255, 0.8)); }
}

/* Promo Tooltip */
.ai-promo-tooltip {
  position: absolute;
  bottom: 70px;
  right: 0;
  width: 220px;
  background: linear-gradient(135deg, rgba(30, 27, 75, 0.95) 0%, rgba(49, 46, 129, 0.95) 100%);
  color: #fff;
  padding: 14px 16px;
  border-radius: 14px;
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  border: 1px solid rgba(139, 92, 246, 0.3);
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.3);
}

.ai-promo-tooltip::after {
  content: '';
  position: absolute;
  bottom: -7px;
  right: 22px;
  width: 0;
  height: 0;
  border-left: 7px solid transparent;
  border-right: 7px solid transparent;
  border-top: 7px solid rgba(49, 46, 129, 0.95);
}

.fade-slide-enter-active, .fade-slide-leave-active {
  transition: opacity 0.3s, transform 0.3s;
}
.fade-slide-enter-from, .fade-slide-leave-to {
  opacity: 0;
  transform: translateY(8px) scale(0.96);
}

/* Chat Window */
.ai-chat-window {
  position: fixed;
  border-radius: 20px;
  overflow: hidden;
  border: 1px solid rgba(139, 92, 246, 0.2);
  box-shadow: 0 25px 80px rgba(0, 0, 0, 0.35), 0 0 60px rgba(99, 102, 241, 0.1);
  display: flex;
  flex-direction: column;
  transition: width 0.3s ease, height 0.3s ease, border-radius 0.3s ease;
}

.ai-window-light {
  background: #ffffff;
}

.ai-window-dark {
  background: rgba(15, 15, 45, 0.95);
  border-color: rgba(139, 92, 246, 0.25);
}

.ai-chat-window.is-resizing {
  transition: none;
}

/* Header Styles */
.ai-header-dark {
  background: linear-gradient(135deg, #1e1b4b 0%, #312e81 100%);
  border-bottom: 1px solid rgba(139, 92, 246, 0.2);
}

.ai-header-light {
  background: linear-gradient(135deg, #f5f3ff 0%, #ede9fe 100%);
  border-bottom: 1px solid rgba(139, 92, 246, 0.15);
}

.ai-avatar {
  width: 36px;
  height: 36px;
  border-radius: 12px;
  background: linear-gradient(135deg, #6366f1 0%, #a855f7 100%);
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  font-size: 1rem;
  box-shadow: 0 2px 10px rgba(99, 102, 241, 0.3);
}

.ai-status-text {
  font-size: 0.7rem;
  display: flex;
  align-items: center;
  gap: 4px;
}

.ai-status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #22c55e;
  animation: status-pulse 2s infinite;
}

@keyframes status-pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}

.ai-btn-icon {
  width: 30px;
  height: 30px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: none;
  background: transparent;
  color: inherit;
  transition: background-color 0.2s, color 0.2s;
}

.ai-header-dark .ai-btn-icon {
  color: rgba(255, 255, 255, 0.8);
}

.ai-header-dark .ai-btn-icon:hover {
  background: rgba(255, 255, 255, 0.1);
  color: #fff;
}

.ai-header-light .ai-btn-icon {
  color: rgba(30, 27, 75, 0.7);
}

.ai-header-light .ai-btn-icon:hover {
  background: rgba(139, 92, 246, 0.1);
  color: #6366f1;
}

.ai-btn-close:hover {
  background: rgba(239, 68, 68, 0.2) !important;
  color: #ef4444 !important;
}

.ai-drag-handle {
  cursor: grab;
}
.ai-drag-handle:active {
  cursor: grabbing;
}

/* Empty State */
.ai-empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 40px 20px;
  min-height: 280px;
}

.ai-empty-icon {
  width: 72px;
  height: 72px;
  border-radius: 24px;
  background: linear-gradient(135deg, rgba(99, 102, 241, 0.1) 0%, rgba(168, 85, 247, 0.1) 100%);
  display: flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 20px;
}

.ai-empty-icon i {
  font-size: 2rem;
  background: linear-gradient(135deg, #6366f1 0%, #a855f7 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}

.ai-empty-title {
  font-size: 1.1rem;
  color: inherit;
}

.ai-empty-hint {
  color: rgba(128, 128, 128, 0.8);
  font-size: 0.85rem;
  text-align: center;
  max-width: 280px;
}

.ai-quick-prompts {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 8px;
}

.ai-quick-prompt-btn {
  border-radius: 20px;
  padding: 6px 14px;
  font-size: 0.8rem;
  border: 1px solid rgba(99, 102, 241, 0.3);
  background: rgba(99, 102, 241, 0.05);
  color: #6366f1;
  transition: all 0.2s;
}

.ai-quick-prompt-btn:hover {
  background: linear-gradient(135deg, #6366f1 0%, #8b5cf6 100%);
  color: #fff;
  border-color: transparent;
  transform: translateY(-1px);
}

/* Message Bubbles */
.ai-message-wrapper {
  animation: msg-appear 0.3s ease;
}

@keyframes msg-appear {
  from { opacity: 0; transform: translateY(8px); }
  to { opacity: 1; transform: translateY(0); }
}

.ai-user-bubble {
  max-width: 80%;
  padding: 12px 16px;
  border-radius: 18px 18px 4px 18px;
  background: linear-gradient(135deg, #6366f1 0%, #8b5cf6 100%);
  color: #fff;
  box-shadow: 0 2px 12px rgba(99, 102, 241, 0.3);
}

.ai-bubble-content {
  line-height: 1.5;
  word-break: break-word;
}

.ai-file-badge {
  display: inline-flex;
  align-items: center;
  padding: 2px 10px;
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.2);
  font-size: 0.75rem;
  margin-right: 4px;
  margin-bottom: 4px;
}

.ai-assistant-bubble {
  max-width: 85%;
  padding: 12px 16px;
  border-radius: 18px 18px 18px 4px;
  background: rgba(255, 255, 255, 0.9);
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.08);
}

.ai-window-dark .ai-assistant-bubble {
  background: rgba(30, 27, 75, 0.6);
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.3);
}

.ai-window-dark .ai-assistant-name {
  color: rgba(255, 255, 255, 0.9);
}

.ai-window-dark .ai-reasoning-block {
  background: rgba(99, 102, 241, 0.1);
  border-color: rgba(99, 102, 241, 0.2);
}

.ai-window-dark .ai-reasoning-collapsed {
  background: rgba(99, 102, 241, 0.05);
}

.ai-window-dark .ai-reasoning-header {
  color: #a78bfa;
}

.ai-window-dark .ai-reasoning-content {
  color: rgba(255, 255, 255, 0.6);
}

.ai-assistant-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 10px;
}

.ai-markdown {
  padding: 0 4px;
  line-height: 1.6;
}

.ai-window-dark .ai-markdown {
  color: rgba(255, 255, 255, 0.9);
}

.ai-window-light .ai-markdown {
  color: #1e1b4b;
}

.ai-window-dark .ai-markdown code {
  color: #e2e8f0;
  background: rgba(255, 255, 255, 0.1);
}

.ai-assistant-avatar {
  width: 28px;
  height: 28px;
  border-radius: 8px;
  background: linear-gradient(135deg, #6366f1 0%, #a855f7 100%);
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  font-size: 0.75rem;
}

.ai-assistant-name {
  font-size: 0.8rem;
  font-weight: 600;
  color: #1e1b4b;
}

.ai-reasoning-block {
  border-radius: 10px;
  background: rgba(139, 92, 246, 0.05);
  border: 1px solid rgba(139, 92, 246, 0.1);
  overflow: hidden;
}

.ai-reasoning-header {
  display: flex;
  align-items: center;
  font-size: 0.8rem;
  font-weight: 500;
  color: #6366f1;
  padding: 8px 12px;
}

.ai-reasoning-content-wrapper {
  max-height: 500px;
  overflow: hidden;
  transition: max-height 0.3s ease;
}

.ai-reasoning-collapsed .ai-reasoning-content-wrapper {
  max-height: 0;
}

.ai-reasoning-content {
  padding: 0 12px 12px 32px;
  font-size: 0.8rem;
  color: #64748b;
  font-style: italic;
  border-left: 2px solid rgba(139, 92, 246, 0.2);
}

.ai-thinking-indicator {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #f59e0b;
  margin-left: 6px;
  animation: think-pulse 1s infinite;
}

@keyframes think-pulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.5; transform: scale(0.8); }
}

/* Action Plan */
.ai-action-plan {
  margin-top: 16px;
  border-radius: 12px;
  border: 1px solid rgba(139, 92, 246, 0.15);
  background: rgba(249, 250, 251, 0.8);
  overflow: hidden;
}

.ai-action-plan-header {
  display: flex;
  align-items: center;
  padding: 12px 14px;
  font-size: 0.85rem;
  font-weight: 600;
  color: #1e1b4b;
  background: rgba(139, 92, 246, 0.05);
  border-bottom: 1px solid rgba(139, 92, 246, 0.1);
}

.ai-action-status-badge {
  margin-left: auto;
  padding: 2px 10px;
  border-radius: 10px;
  font-size: 0.7rem;
  font-weight: 500;
}

.ai-status-pending {
  background: rgba(245, 158, 11, 0.15);
  color: #d97706;
}

.ai-status-executed {
  background: rgba(34, 197, 94, 0.15);
  color: #16a34a;
}

.ai-action-list {
  padding: 12px 14px;
  margin: 0;
  list-style: none;
}

.ai-action-item {
  padding: 8px 0;
  border-bottom: 1px dashed rgba(139, 92, 246, 0.08);
  font-size: 0.85rem;
  line-height: 1.5;
}

.ai-action-item:last-child {
  border-bottom: none;
}

.ai-action-tag {
  display: inline-block;
  padding: 1px 8px;
  border-radius: 6px;
  font-size: 0.7rem;
  font-weight: 600;
  margin-right: 6px;
}

.ai-action-create_product { background: rgba(99, 102, 241, 0.12); color: #6366f1; }
.ai-action-create_device { background: rgba(34, 197, 94, 0.12); color: #16a34a; }
.ai-action-update_mapping { background: rgba(6, 182, 212, 0.12); color: #0891b2; }
.ai-action-update_product, .ai-action-update_device { background: rgba(99, 102, 241, 0.08); color: #4f46e5; }
.ai-action-delete_product, .ai-action-delete_device { background: rgba(239, 68, 68, 0.1); color: #dc2626; }

.ai-action-footer {
  padding: 10px 14px;
  display: flex;
  justify-content: flex-end;
  background: rgba(139, 92, 246, 0.03);
  border-top: 1px solid rgba(139, 92, 246, 0.08);
}

.ai-execute-btn {
  border-radius: 8px;
  padding: 6px 16px;
  font-size: 0.8rem;
  background: linear-gradient(135deg, #6366f1 0%, #8b5cf6 100%);
  color: #fff;
  border: none;
  transition: all 0.2s;
}

.ai-execute-btn:hover:not(:disabled) {
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(99, 102, 241, 0.3);
}

.ai-execute-btn:disabled {
  opacity: 0.6;
}

.ai-exec-success {
  display: flex;
  align-items: center;
  font-size: 0.8rem;
  color: #16a34a;
}

/* Typing Indicator */
.ai-typing-indicator {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 4px 0;
}

.ai-typing-avatar {
  width: 28px;
  height: 28px;
  border-radius: 8px;
  background: linear-gradient(135deg, #6366f1 0%, #a855f7 100%);
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  font-size: 0.75rem;
}

.ai-typing-dots {
  display: flex;
  gap: 4px;
  padding: 8px 12px;
  border-radius: 14px;
  background: rgba(139, 92, 246, 0.08);
}

.ai-typing-dots span {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #8b5cf6;
  animation: typing-bounce 1.4s infinite ease-in-out;
}

.ai-typing-dots span:nth-child(1) { animation-delay: 0s; }
.ai-typing-dots span:nth-child(2) { animation-delay: 0.2s; }
.ai-typing-dots span:nth-child(3) { animation-delay: 0.4s; }

@keyframes typing-bounce {
  0%, 60%, 100% { transform: translateY(0); opacity: 0.4; }
  30% { transform: translateY(-4px); opacity: 1; }
}

/* Input Area */
.ai-input-area {
  padding: 12px 14px;
  border-top: 1px solid rgba(139, 92, 246, 0.1);
  background: rgba(249, 250, 251, 0.5);
}

.ai-attachments-preview {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-bottom: 10px;
}

.ai-attachment-badge {
  display: inline-flex;
  align-items: center;
  padding: 4px 10px;
  border-radius: 8px;
  background: rgba(99, 102, 241, 0.08);
  border: 1px solid rgba(99, 102, 241, 0.12);
  font-size: 0.75rem;
  gap: 6px;
}

.ai-attachment-thumb {
  width: 20px;
  height: 20px;
  border-radius: 4px;
  object-fit: cover;
}

.ai-attachment-name {
  max-width: 120px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ai-attachment-remove {
  cursor: pointer;
  opacity: 0.6;
  transition: opacity 0.2s;
}

.ai-attachment-remove:hover {
  opacity: 1;
}

.ai-input-wrapper {
  border-radius: 14px;
  overflow: hidden;
  transition: box-shadow 0.2s;
}

.ai-input-dark {
  background: rgba(30, 27, 75, 0.6);
  border: 1px solid rgba(139, 92, 246, 0.2);
}

.ai-input-light {
  background: #fff;
  border: 1px solid rgba(139, 92, 246, 0.15);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.04);
}

.ai-textarea {
  width: 100%;
  border: none;
  background: transparent;
  padding: 12px 14px;
  font-size: 0.9rem;
  line-height: 1.5;
  resize: none;
  outline: none;
}

.ai-input-dark .ai-textarea {
  color: rgba(255, 255, 255, 0.9);
}

.ai-input-light .ai-textarea {
  color: #1e1b4b;
}

.ai-textarea::placeholder {
  opacity: 0.5;
}

.ai-input-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  border-top: 1px solid rgba(139, 92, 246, 0.08);
}

.ai-input-tools {
  display: flex;
  align-items: center;
  gap: 8px;
}

.ai-tool-btn {
  width: 32px;
  height: 32px;
  border-radius: 8px;
  border: none;
  background: transparent;
  color: #64748b;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s;
}

.ai-tool-btn:hover:not(:disabled) {
  background: rgba(99, 102, 241, 0.1);
  color: #6366f1;
}

.ai-model-selector {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 4px 8px;
  border-radius: 6px;
  background: rgba(99, 102, 241, 0.06);
  font-size: 0.75rem;
  color: #6366f1;
}

.ai-model-select {
  border: none;
  background: transparent;
  font-size: 0.75rem;
  color: #6366f1;
  outline: none;
  cursor: pointer;
}

.ai-send-btn {
  border-radius: 10px;
  padding: 8px 18px;
  font-size: 0.85rem;
  background: linear-gradient(135deg, #6366f1 0%, #8b5cf6 100%);
  color: #fff;
  border: none;
  font-weight: 500;
  transition: all 0.2s;
  display: flex;
  align-items: center;
}

.ai-send-btn:hover:not(:disabled) {
  transform: translateY(-1px);
  box-shadow: 0 4px 14px rgba(99, 102, 241, 0.4);
}

.ai-send-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

/* Resize Handles */
.resize-handle {
  position: absolute;
  z-index: 10001;
}
.resize-handle.n { top: -3px; left: 10px; right: 10px; height: 6px; cursor: n-resize; }
.resize-handle.s { bottom: -3px; left: 10px; right: 10px; height: 6px; cursor: s-resize; }
.resize-handle.e { top: 10px; bottom: 10px; right: -3px; width: 6px; cursor: e-resize; }
.resize-handle.w { top: 10px; bottom: 10px; left: -3px; width: 6px; cursor: w-resize; }
.resize-handle.ne { top: -3px; right: -3px; width: 10px; height: 10px; cursor: ne-resize; }
.resize-handle.nw { top: -3px; left: -3px; width: 10px; height: 10px; cursor: nw-resize; }
.resize-handle.se { bottom: -3px; right: -3px; width: 15px; height: 15px; cursor: se-resize;
  background: linear-gradient(135deg, transparent 50%, rgba(139, 92, 246, 0.2) 50%); }
.resize-handle.sw { bottom: -3px; left: -3px; width: 10px; height: 10px; cursor: sw-resize; }

/*<!-- Chat Messages Container -->
.chat-messages {
  font-size: 0.9rem;
}

.ai-window-light .chat-messages {
  background-color: #f8fafc;
}

.ai-window-dark .chat-messages {
  background-color: #0f0f2d;
}

/* Dark Mode Adjustments */
.ai-window-dark .ai-assistant-name {
  color: rgba(255, 255, 255, 0.9);
}

.ai-window-dark .ai-action-plan {
  background: rgba(30, 27, 75, 0.5);
  border-color: rgba(139, 92, 246, 0.3);
}

.ai-window-dark .ai-action-plan-header {
  color: rgba(255, 255, 255, 0.9);
  background: rgba(139, 92, 246, 0.15);
  border-bottom-color: rgba(139, 92, 246, 0.2);
}

.ai-window-dark .ai-action-item {
  color: rgba(255, 255, 255, 0.85);
  border-bottom-color: rgba(139, 92, 246, 0.15);
}

.ai-window-dark .ai-action-item code {
  color: #c4b5fd !important;
  background: rgba(139, 92, 246, 0.25) !important;
  padding: 2px 6px;
  border-radius: 4px;
}

.ai-window-dark .ai-action-item .text-muted {
  color: rgba(255, 255, 255, 0.5) !important;
}

.ai-window-dark .ai-action-footer {
  background: rgba(139, 92, 246, 0.08);
  border-top-color: rgba(139, 92, 246, 0.15);
}

.ai-window-dark .ai-action-create_product { color: #818cf8; }
.ai-window-dark .ai-action-create_device { color: #4ade80; }
.ai-window-dark .ai-action-update_mapping { color: #22d3ee; }
.ai-window-dark .ai-action-update_product, 
.ai-window-dark .ai-action-update_device { color: #818cf8; }
.ai-window-dark .ai-action-delete_product, 
.ai-window-dark .ai-action-delete_device { color: #f87171; }

.ai-window-dark .ai-attachment-badge {
  background: rgba(99, 102, 241, 0.2);
  color: rgba(255, 255, 255, 0.9);
}

.ai-window-dark .ai-input-area {
  background: rgba(15, 15, 45, 0.8);
}
</style>
