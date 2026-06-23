<template>
  <div class="var-picker-popover shadow border rounded bg-white" @click.stop>
    <div class="d-flex h-100">
      <!-- Left Pane: Categories / Nodes -->
      <div class="category-pane border-end p-2 overflow-auto" style="width: 150px;">
        <div class="fw-bold text-muted small mb-2 px-1">数据来源</div>
        
        <!-- Trigger Node -->
        <div 
          class="category-item rounded p-1 mb-1" 
          :class="{ 'bg-primary text-white': activeCategory === 'trigger' }"
          @click="activeCategory = 'trigger'"
        >
          <i class="bi bi-lightning-charge me-1"></i>触发器数据
        </div>
        
        <!-- System Variables -->
        <div 
          class="category-item rounded p-1 mb-2" 
          :class="{ 'bg-primary text-white': activeCategory === 'system' }"
          @click="activeCategory = 'system'"
        >
          <i class="bi bi-gear me-1"></i>系统变量
        </div>

        <!-- Action Nodes -->
        <div v-if="actionNodes.length > 0" class="fw-bold text-muted small mb-2 px-1 mt-3">前置动作结果</div>
        <div 
          v-for="node in actionNodes" 
          :key="node.id"
          class="category-item rounded p-1 mb-1 text-truncate" 
          :class="{ 'bg-primary text-white': activeCategory === node.id }"
          @click="activeCategory = node.id"
          :title="node.name || node.type"
        >
          <i class="bi bi-play-circle me-1"></i>{{ node.name || node.type }}
        </div>
      </div>

      <!-- Right Pane: Variables -->
      <div class="variable-pane p-2 overflow-auto" style="flex: 1;">
        <div class="fw-bold text-muted small mb-2 px-1">可用变量</div>
        
        <template v-if="activeCategory === 'trigger'">
          <div class="var-item rounded p-1 mb-1 d-flex justify-content-between align-items-center" @click="insert('${trigger.deviceName}')">
            <span>设备名称</span> <code class="small text-muted">${trigger.deviceName}</code>
          </div>
          <div class="var-item rounded p-1 mb-1 d-flex justify-content-between align-items-center" @click="insert('${trigger.deviceCode}')">
            <span>设备编码</span> <code class="small text-muted">${trigger.deviceCode}</code>
          </div>
          <div class="var-item rounded p-1 mb-1 d-flex justify-content-between align-items-center" @click="insert('${event.payload}')">
            <span>事件原始负载 (JSON)</span> <code class="small text-muted">${event.payload}</code>
          </div>
          <div class="var-item rounded p-1 mb-1 d-flex justify-content-between align-items-center" @click="insert('${trigger.properties.temperature}')">
            <span>设备温度 (示例)</span> <code class="small text-muted">${trigger.properties.temperature}</code>
          </div>
          <div class="var-item rounded p-1 mb-1 d-flex justify-content-between align-items-center" @click="insert('${trigger.properties.humidity}')">
            <span>设备湿度 (示例)</span> <code class="small text-muted">${trigger.properties.humidity}</code>
          </div>
        </template>

        <template v-else-if="activeCategory === 'system'">
          <div class="var-item rounded p-1 mb-1 d-flex justify-content-between align-items-center" @click="insert('${rule.name}')">
            <span>规则名称</span> <code class="small text-muted">${rule.name}</code>
          </div>
          <div class="var-item rounded p-1 mb-1 d-flex justify-content-between align-items-center" @click="insert('${event.timestamp}')">
            <span>触发时间戳</span> <code class="small text-muted">${event.timestamp}</code>
          </div>
        </template>

        <template v-else>
          <!-- Specific Node Output -->
          <div class="var-item rounded p-1 mb-1 d-flex justify-content-between align-items-center" @click="insert(`\${node.${activeCategory}.result}`)">
            <span>原始返回结果</span> <code class="small text-muted">${node.{{ activeCategory }}.result}</code>
          </div>
          <div class="var-item rounded p-1 mb-1 d-flex justify-content-between align-items-center" @click="insert(`\${node.${activeCategory}.error}`)">
            <span>错误信息</span> <code class="small text-muted">${node.{{ activeCategory }}.error}</code>
          </div>
          <div class="var-item rounded p-1 mb-1 d-flex justify-content-between align-items-center" @click="insert(`\${node.${activeCategory}.result.data}`)">
            <span>解析 JSON data 字段</span> <code class="small text-muted">${node.{{ activeCategory }}.result.data}</code>
          </div>
        </template>
        
        <div class="mt-3 small text-muted px-1">
          <i class="bi bi-info-circle me-1"></i>点击变量即可插入。支持使用 <code>.</code> 访问 JSON 对象的深层属性。
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { defineComponent, ref, inject, computed } from 'vue'

export default defineComponent({
  name: 'VarPickerPopover',
  emits: ['select'],
  setup(props, { emit }) {
    const activeCategory = ref('trigger')
    
    // Attempt to inject ruleContext (provided by RuleEngine or RuleGraphViewer)
    const ruleContext = inject('ruleContext', null)
    
    // Extract actions to provide them as previous nodes
    const actionNodes = computed(() => {
      if (!ruleContext || !ruleContext.actions) return []
      return extractNodes(ruleContext.actions)
    })
    
    const extractNodes = (actions) => {
      let nodes = []
      for (const act of actions) {
        if (act.type === 'sequence_group' || act.type === 'parallel_group') {
           if (act.subActions) {
             nodes.push(...extractNodes(act.subActions))
           }
        } else {
           nodes.push(act)
        }
      }
      return nodes
    }

    const insert = (val) => {
      emit('select', val)
    }

    return {
      activeCategory,
      actionNodes,
      insert
    }
  }
})
</script>

<style scoped>
.var-picker-popover {
  position: absolute;
  right: 0;
  bottom: 30px;
  z-index: 1060;
  width: 450px;
  height: 250px;
  font-size: 0.85rem;
  box-shadow: 0 10px 15px -3px rgba(0,0,0,0.1), 0 4px 6px -2px rgba(0,0,0,0.05) !important;
}

.category-item {
  cursor: pointer;
  transition: all 0.2s;
  color: var(--bs-body-color);
}
.category-item:hover:not(.bg-primary) {
  background-color: rgba(139, 92, 246, 0.1);
  color: #8b5cf6;
}

.var-item {
  cursor: pointer;
  transition: all 0.2s;
}
.var-item:hover {
  background-color: rgba(139, 92, 246, 0.1);
}
.var-item code {
  color: #8b5cf6;
}

[data-bs-theme="dark"] .var-picker-popover {
  background-color: #1e293b !important;
  border-color: #334155 !important;
}
[data-bs-theme="dark"] .category-item { color: #cbd5e1; }
[data-bs-theme="dark"] .var-item:hover { background-color: rgba(139, 92, 246, 0.2); }
</style>
