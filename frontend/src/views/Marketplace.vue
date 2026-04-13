<template>
  <div v-if="loading" class="d-flex justify-content-center py-5">
    <div class="spinner-border text-primary" role="status">
      <span class="visually-hidden">{{ $t('loading') }}</span>
    </div>
  </div>

  <div v-else>
    <!-- Platform Plugins -->
    <div v-if="platforms.length > 0">
      <h5 class="mb-3 text-secondary border-bottom pb-2">{{ $t('cat_platform_plugins') }}</h5>
      <div class="row g-4 mb-5">
        <div v-for="plugin in platforms" :key="plugin.name" class="col-md-6 col-lg-4 col-xl-3">
          <PluginCard :plugin="plugin" @configure="$emit('configure', plugin.name)" @update-status="handleStatusUpdate" />
        </div>
      </div>
    </div>

    <!-- Protocol Plugins -->
    <div v-if="protocols.length > 0">
      <h5 class="mb-3 text-secondary border-bottom pb-2">{{ $t('cat_protocol_plugins') }}</h5>
      <div class="row g-4 mb-5">
        <div v-for="plugin in protocols" :key="plugin.name" class="col-md-6 col-lg-4 col-xl-3">
          <PluginCard :plugin="plugin" @configure="$emit('configure', plugin.name)" @update-status="handleStatusUpdate" />
        </div>
      </div>
    </div>

    <!-- Other Plugins -->
    <div v-if="others.length > 0">
      <h5 class="mb-3 text-secondary border-bottom pb-2">{{ $t('cat_other_plugins') }}</h5>
      <div class="row g-4 mb-5">
        <div v-for="plugin in others" :key="plugin.name" class="col-md-6 col-lg-4 col-xl-3">
          <PluginCard :plugin="plugin" @configure="$emit('configure', plugin.name)" @update-status="handleStatusUpdate" />
        </div>
      </div>
    </div>
    
    <div v-if="plugins.length === 0" class="alert alert-info">{{ $t('no_active_plugins') }}</div>
  </div>
</template>

<script setup>
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';
import PluginCard from '../components/PluginCard.vue';

const props = defineProps({
  plugins: Array,
  loading: Boolean
});

const emit = defineEmits(['configure', 'update-status']);

const platforms = computed(() => props.plugins.filter(p => p.category === 'platform'));
const protocols = computed(() => props.plugins.filter(p => p.category === 'protocol'));
const others = computed(() => props.plugins.filter(p => p.category !== 'platform' && p.category !== 'protocol'));

const handleStatusUpdate = (name, enabled) => {
  emit('update-status', name, enabled);
};
</script>
