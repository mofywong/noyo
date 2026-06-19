<template>
  <aside class="rule-graph-palette">
    <div class="rule-graph-toolbox">
      <div class="toolbox-title">
        <span>{{ title }}</span>
        <i class="bi bi-grip-vertical"></i>
      </div>
      <button
        v-for="item in items"
        :key="item.kind + ':' + item.type"
        type="button"
        class="palette-item"
        draggable="true"
        @dragstart="onDragStart($event, item)"
        @click="$emit('add', item)"
      >
        <span class="palette-item__icon" :class="'palette-item__icon--' + item.kind">
          <i class="bi" :class="item.icon"></i>
        </span>
        <span class="palette-item__body">
          <span class="palette-item__name">{{ item.label }}</span>
          <span class="palette-item__hint">{{ item.hint }}</span>
        </span>
      </button>
    </div>
  </aside>
</template>

<script>
export default {
  name: 'RuleGraphPalette',
  props: {
    title: {
      type: String,
      required: true
    },
    items: {
      type: Array,
      default: () => []
    }
  },
  emits: ['add'],
  setup() {
    function onDragStart(event, item) {
      event.dataTransfer.effectAllowed = 'copy'
      event.dataTransfer.setData('application/json', JSON.stringify(item))
      event.dataTransfer.setData('text/plain', `${item.kind}:${item.type}`)
    }

    return { onDragStart }
  }
}
</script>

<style scoped>
.rule-graph-palette {
  min-width: 0;
}

.rule-graph-toolbox {
  display: grid;
  gap: 0.6rem;
  position: sticky;
  top: 1rem;
}

.toolbox-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  color: var(--bs-secondary-color);
  font-size: 0.78rem;
  font-weight: 700;
  text-transform: uppercase;
}

.palette-item {
  display: flex;
  align-items: center;
  gap: 0.65rem;
  width: 100%;
  border: 1px solid var(--bs-border-color);
  border-radius: 8px;
  padding: 0.7rem;
  color: var(--bs-body-color);
  background: var(--bs-body-bg);
  text-align: left;
  transition: border-color 0.16s ease, box-shadow 0.16s ease, transform 0.16s ease;
}

.palette-item:hover {
  border-color: var(--bs-primary);
  box-shadow: 0 0.35rem 1rem rgba(var(--bs-primary-rgb), 0.1);
  transform: translateY(-1px);
}

.palette-item__icon {
  flex: 0 0 2rem;
  width: 2rem;
  height: 2rem;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 8px;
  color: #fff;
  background: var(--bs-primary);
}

.palette-item__icon--condition {
  background: #6f42c1;
}

.palette-item__icon--action {
  background: #fd7e14;
}

.palette-item__body {
  display: grid;
  gap: 0.15rem;
  min-width: 0;
}

.palette-item__name {
  font-weight: 700;
  line-height: 1.2;
}

.palette-item__hint {
  color: var(--bs-secondary-color);
  font-size: 0.76rem;
  line-height: 1.25;
}
</style>
