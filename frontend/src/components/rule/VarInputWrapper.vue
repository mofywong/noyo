<template>
  <div class="position-relative w-100" ref="menuRef">
    <textarea 
      v-if="textarea"
      :readonly="readonly"
      :disabled="disabled"
      class="form-control form-control-sm" 
      :rows="rows || 3"
      :placeholder="placeholder || ''"
      :value="modelValue" 
      @input="$emit('update:modelValue', $event.target.value)"
    ></textarea>
    <input 
      v-else
      :readonly="readonly"
      :disabled="disabled"
      :type="type || 'text'"
      class="form-control form-control-sm" 
      :placeholder="placeholder || ''"
      :value="modelValue" 
      @input="$emit('update:modelValue', $event.target.value)"
    >

    <i 
      v-if="!readonly && !disabled"
      class="bi bi-magic position-absolute" 
      style="right: 8px; bottom: 8px; cursor: pointer; color: #8b5cf6; background: rgba(255,255,255,0.8); border-radius: 4px; padding: 2px;"
      @click="toggleMenu"
      title="插入变量"
    ></i>

    <VarPickerPopover v-if="showMenu" @select="insertVar" />
  </div>
</template>

<script>
import { defineComponent, ref, onMounted, onUnmounted } from 'vue'
import VarPickerPopover from './VarPickerPopover.vue'

export default defineComponent({
  name: 'VarInputWrapper',
  components: { VarPickerPopover },
  props: ['modelValue', 'textarea', 'rows', 'placeholder', 'type', 'readonly', 'disabled'],
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    const showMenu = ref(false)
    const toggleMenu = () => showMenu.value = !showMenu.value
    const insertVar = (v) => {
      emit('update:modelValue', (props.modelValue || '') + v)
      showMenu.value = false
    }
    
    const menuRef = ref(null)
    const handleClickOutside = (e) => {
      if (showMenu.value && menuRef.value && !menuRef.value.contains(e.target)) {
        showMenu.value = false
      }
    }
    
    onMounted(() => document.addEventListener('click', handleClickOutside, true))
    onUnmounted(() => document.removeEventListener('click', handleClickOutside, true))

    return {
      showMenu, toggleMenu, insertVar, menuRef
    }
  }
})
</script>
