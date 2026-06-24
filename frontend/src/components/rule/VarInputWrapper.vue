<template>
  <div class="position-relative w-100" ref="menuRef">
    <textarea 
      v-if="textarea"
      ref="textareaRef"
      :readonly="readonly"
      :disabled="disabled"
      :class="['form-control form-control-sm var-textarea', canShowMagic ? 'var-input-has-magic' : '']" 
      :rows="rows || 3"
      :placeholder="placeholder || ''"
      :value="modelValue" 
      @focus="updateSelection"
      @click="updateSelection"
      @keyup="updateSelection"
      @select="updateSelection"
      @input="handleTextareaInput"
    ></textarea>
    <input 
      v-else
      ref="inputRef"
      :readonly="readonly"
      :disabled="disabled"
      :type="type || 'text'"
      :class="['form-control form-control-sm', canShowMagic ? 'var-input-has-magic' : '']" 
      :placeholder="placeholder || ''"
      :value="modelValue" 
      @focus="updateSelection"
      @click="updateSelection"
      @keyup="updateSelection"
      @select="updateSelection"
      @input="handleInput"
    >

    <i 
      v-if="canShowMagic"
      class="bi bi-magic position-absolute var-magic-btn" 
      @mousedown.prevent="updateSelection"
      @click="openMenu"
      title="插入变量"
    ></i>

    <Teleport to="body">
      <VarPickerPopover v-if="showMenu" :position-style="popoverStyle" @select="insertVar" @close="closeMenu" />
    </Teleport>
  </div>
</template>

<script>
import { computed, defineComponent, inject, ref, onMounted, onUnmounted, nextTick, watch } from 'vue'
import VarPickerPopover from './VarPickerPopover.vue'

export default defineComponent({
  name: 'VarInputWrapper',
  components: { VarPickerPopover },
  props: ['modelValue', 'textarea', 'rows', 'placeholder', 'type', 'readonly', 'disabled', 'autoGrow', 'maxRows'],
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    const showMenu = ref(false)
    const popoverStyle = ref({})
    const selection = ref(null)
    const ruleContext = inject('ruleContext', null)
    const canShowMagic = computed(() => {
      if (props.readonly || props.disabled) return false
      if (!ruleContext?.hasReferenceContent) return false
      return ruleContext.hasReferenceContent()
    })
    const updatePopoverPosition = () => {
      if (!menuRef.value) return
      const rect = menuRef.value.getBoundingClientRect()
      const margin = 12
      const width = Math.min(760, window.innerWidth - margin * 2)
      const height = window.innerWidth <= 720 ? Math.min(480, window.innerHeight - margin * 2) : 360
      const left = Math.min(Math.max(rect.right - width, margin), window.innerWidth - width - margin)
      let top = rect.top - height - 8
      if (top < margin) top = Math.min(rect.bottom + 8, window.innerHeight - height - margin)
      popoverStyle.value = {
        left: `${Math.max(margin, left)}px`,
        top: `${Math.max(margin, top)}px`,
        width: `${width}px`,
        height: `${height}px`
      }
    }
    const openMenu = () => {
      if (document.activeElement !== fieldEl()) selection.value = null
      showMenu.value = true
      nextTick(updatePopoverPosition)
    }
    const closeMenu = () => {
      showMenu.value = false
    }
    const fieldEl = () => props.textarea ? textareaRef.value : inputRef.value
    const updateSelection = (event) => {
      const el = fieldEl()
      if (!el || typeof el.selectionStart !== 'number') return
      if (event?.target !== el && document.activeElement !== el) return
      selection.value = {
        start: el.selectionStart,
        end: el.selectionEnd ?? el.selectionStart
      }
    }
    const insertVar = (v) => {
      const current = props.modelValue || ''
      const range = selection.value || { start: current.length, end: current.length }
      const start = Math.min(Math.max(range.start, 0), current.length)
      const end = Math.min(Math.max(range.end, start), current.length)
      const nextValue = `${current.slice(0, start)}${v}${current.slice(end)}`
      const cursor = start + v.length
      emit('update:modelValue', nextValue)
      selection.value = { start: cursor, end: cursor }
      nextTick(() => {
        resizeTextarea()
        const el = fieldEl()
        if (el) {
          el.focus()
          if (typeof el.setSelectionRange === 'function') el.setSelectionRange(cursor, cursor)
        }
      })
    }
    
    const menuRef = ref(null)
    const textareaRef = ref(null)
    const inputRef = ref(null)
    const resizeTextarea = () => {
      if (!props.textarea || props.autoGrow === false || !textareaRef.value) return
      const el = textareaRef.value
      const lineHeight = parseFloat(window.getComputedStyle(el).lineHeight) || 20
      const maxRows = Number(props.maxRows || 12)
      el.style.height = 'auto'
      el.style.height = `${Math.min(el.scrollHeight, lineHeight * maxRows + 18)}px`
      el.style.overflowY = el.scrollHeight > lineHeight * maxRows + 18 ? 'auto' : 'hidden'
    }
    const handleTextareaInput = (e) => {
      emit('update:modelValue', e.target.value)
      updateSelection()
      nextTick(resizeTextarea)
    }
    const handleInput = (e) => {
      emit('update:modelValue', e.target.value)
      updateSelection()
    }
    
    onMounted(() => {
      window.addEventListener('resize', updatePopoverPosition)
      window.addEventListener('scroll', updatePopoverPosition, true)
      nextTick(resizeTextarea)
    })
    onUnmounted(() => {
      window.removeEventListener('resize', updatePopoverPosition)
      window.removeEventListener('scroll', updatePopoverPosition, true)
    })
    watch(() => props.modelValue, () => nextTick(resizeTextarea))
    watch(() => props.rows, () => nextTick(resizeTextarea))

    return {
      showMenu, openMenu, closeMenu, insertVar, menuRef, textareaRef, inputRef, handleTextareaInput, handleInput, updateSelection, popoverStyle, canShowMagic
    }
  }
})
</script>

<style scoped>
.var-textarea {
  min-height: calc(1.5em * var(--var-input-min-rows, 3) + 0.5rem + 2px);
  resize: vertical;
}
.var-input-has-magic {
  padding-right: 2rem;
}
.var-magic-btn {
  top: 0.42rem;
  right: 0.55rem;
  z-index: 2;
  cursor: pointer;
  color: #8b5cf6;
  background: rgba(255, 255, 255, 0.9);
  border-radius: 4px;
  padding: 2px;
  line-height: 1;
}
[data-bs-theme="dark"] .var-magic-btn {
  background: rgba(30, 41, 59, 0.9);
}
</style>
