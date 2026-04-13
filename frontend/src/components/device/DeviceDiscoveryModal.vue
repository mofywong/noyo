<template>
  <div class="modal fade show d-block" style="background: rgba(0,0,0,0.5)">
    <div class="modal-dialog modal-xl">
      <div class="modal-content" style="height: 90vh; display: flex; flex-direction: column;">
        <div class="modal-header">
          <h5 class="modal-title">{{ $t('discover_devices') }}</h5>
          <button type="button" class="btn-close" @click="$emit('close')"></button>
        </div>
        <div class="modal-body d-flex flex-column">
          <!-- Scan Controls -->
          <div class="row g-2 mb-3 align-items-center">
            <div class="col-auto">
              <label class="form-label mb-0">{{ $t('protocol') }}:</label>
            </div>
            <div class="col-auto">
              <select class="form-select" v-model="selectedProtocol">
                <option value="BACnet">BACnet/IP</option>
                <!-- Add other protocols if they support discovery -->
              </select>
            </div>
            <div class="col-auto">
               <button class="btn btn-primary" @click="startScan" :disabled="scanning">
                 <span v-if="scanning" class="spinner-border spinner-border-sm me-1"></span>
                 {{ scanning ? $t('scanning') : $t('start_scan') }}
               </button>
            </div>
          </div>

          <!-- Results List -->
          <div class="flex-grow-1 overflow-auto border-top pt-2">
            <div v-if="results.length === 0" class="text-center text-muted py-5">
              {{ hasScanned ? $t('no_devices_found') : $t('click_scan_hint') }}
            </div>
            <table v-else class="table table-hover align-middle">
              <thead class="bg-light sticky-top">
                <tr>
                  <th>{{ $t('dev_ip') }}</th>
                  <th>{{ $t('dev_id') }}</th>
                  <th>{{ $t('dev_name') }}</th>
                  <th>{{ $t('dev_vendor') }}</th>
                  <th>{{ $t('bind_product') }}</th>
                  <th>{{ $t('action') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="(dev, index) in results" :key="index">
                   <td>{{ dev.Config.ip }}:{{ dev.Config.port }}</td>
                   <td>{{ dev.Config.device_id }}</td>
                   <td>{{ dev.Name }}</td>
                   <td>{{ dev.Config.vendor_id || '-' }}</td>
                   <td>
                     <select class="form-select form-select-sm" v-model="dev.bindProductCode">
                        <option value="">{{ $t('select_product') }}</option>
                        <option v-for="p in products" :key="p.code" :value="p.code">{{ p.name }}</option>
                     </select>
                   </td>
                   <td>
                     <button class="btn btn-sm btn-success" @click="addDevice(dev)" :disabled="!dev.bindProductCode || dev.added">
                        <i class="bi bi-plus-lg"></i> {{ dev.added ? $t('added') : $t('add') }}
                     </button>
                   </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue';
import axios from 'axios';
import { useI18n } from 'vue-i18n';

const { t } = useI18n();
const emit = defineEmits(['close', 'device-added']);

const selectedProtocol = ref('BACnet');
const scanning = ref(false);
const hasScanned = ref(false);
const results = ref([]);
const products = ref([]);

// Fetch products for binding
const fetchProducts = async () => {
    try {
        const res = await axios.get('/api/products?pageSize=1000');
        if (res.data.code === 0) {
            products.value = res.data.data;
        }
    } catch (e) {
        console.error(e);
    }
};

onMounted(() => {
    fetchProducts();
});

const startScan = async () => {
    scanning.value = true;
    hasScanned.value = false;
    results.value = [];
    try {
        const res = await axios.post(`/api/plugins/${selectedProtocol.value}/discover`, {});
        if (res.data.code === 0) {
            results.value = res.data.data || [];
        } else {
            alert(res.data.message);
        }
    } catch (e) {
        alert(e.message);
    } finally {
        scanning.value = false;
        hasScanned.value = true;
    }
};

const addDevice = async (dev) => {
    if (!dev.bindProductCode) return;
    
    // Construct new device object
    const newDevice = {
        code: `dev_${Date.now()}_${dev.Config.device_id}`, // Generate code
        name: dev.Name,
        product_code: dev.bindProductCode,
        enabled: true,
        parent_code: '', // Direct connection usually
        config: {
            ip: dev.Config.ip,
            port: dev.Config.port,
            device_id: dev.Config.device_id
        }
    };
    
    // Convert config to JSON string if needed? No, API accepts object in Config field?
    // Wait, backend expects `Config` as string (JSON) or map?
    // In `handleCreateDevice`: `json.Unmarshal(r.GetBody(), &d)`. Config is string in `store.Device`.
    // But frontend usually sends JSON object and backend handles it?
    // `store.Device` struct has `Config string`.
    // So we must stringify it.
    
    const payload = {
        ...newDevice,
        config: JSON.stringify(newDevice.config)
    };

    try {
        const res = await axios.post('/api/devices', payload);
        if (res.data.code === 0) {
            dev.added = true;
            emit('device-added');
        } else {
            alert(res.data.message);
        }
    } catch (e) {
        alert(e.message);
    }
};
</script>
