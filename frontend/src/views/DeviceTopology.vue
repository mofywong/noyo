<template>
  <div class="card border-0 shadow-sm h-100">
    <div class="card-header bg-transparent border-0 d-flex justify-content-between align-items-center py-3">
      <h5 class="mb-0">{{ $t('sidebar_topology') }}</h5>
      <div class="btn-group">
        <button class="btn btn-outline-secondary btn-sm" @click="fitView" :title="$t('tsl_actions')">
          <i class="bi bi-arrows-fullscreen"></i>
        </button>
        <button class="btn btn-outline-secondary btn-sm" @click="refresh" :title="$t('tsl_actions')">
          <i class="bi bi-arrow-clockwise"></i>
        </button>
      </div>
    </div>
    <div class="card-body p-0 position-relative" style="overflow: hidden; min-height: 600px;">
      <div v-if="loading" class="position-absolute top-50 start-50 translate-middle" style="z-index: 10;">
        <div class="spinner-border text-primary" role="status">
          <span class="visually-hidden">Loading...</span>
        </div>
      </div>
      <div id="mountNode" ref="container" style="width: 100%; height: 100%;"></div>
    </div>
  </div>
  
  <!-- Hover Tooltip -->
  <div v-if="tooltipVisible" 
       @mouseenter="onTooltipEnter"
       @mouseleave="onTooltipLeave"
       class="card position-fixed shadow border-0 bg-body-tertiary" 
       :style="{ 
          left: tooltipX + 'px', 
          top: tooltipY + 'px', 
          zIndex: 1000, 
          maxWidth: '300px',
          overflow: 'hidden',
          '--bs-bg-opacity': 0.9,
          backdropFilter: 'blur(10px)'
       }">
    <div class="card-header py-2 border-bottom bg-transparent fw-bold d-flex justify-content-between align-items-center">
      <h6 class="mb-0 text-truncate text-body-emphasis">
        <i class="bi bi-cpu me-2"></i>{{ hoveredDevice?.name || hoveredDevice?.code }}
      </h6>
      <span v-if="hoveredDevice?.online !== undefined" class="badge rounded-pill" :class="hoveredDevice.online ? 'bg-success-subtle text-success border border-success-subtle' : 'bg-secondary-subtle text-secondary border border-secondary-subtle'">
          {{ hoveredDevice.online ? $t('dev_online') : $t('dev_offline') }}
      </span>
    </div>
    <div class="card-body py-2 px-3">
       <div v-if="loadingTooltip" class="text-center py-2">
          <div class="spinner-border spinner-border-sm" role="status"></div>
       </div>
       <div v-else-if="tooltipDataList.length === 0" class="text-muted small">
          {{ $t('tsl_no_data') }}
       </div>
       <div v-else class="small">
          <div v-for="item in tooltipDataList" :key="item.key" class="d-flex justify-content-between mb-1">
             <span class="text-truncate me-3" :title="item.key">{{ item.name }}</span>
             <span class="fw-bold text-nowrap" :class="hoveredDevice?.online ? '' : 'text-warning'">
                {{ item.value }} 
                <span class="text-muted fw-normal" v-if="item.unit">{{ item.unit }}</span>
             </span>
          </div>
       </div>
       <div class="mt-2 pt-2 border-top text-center" v-if="hoveredDevice?.isCamera && hoveredDevice?.online">
          <button class="btn btn-sm btn-primary w-100" @click.stop="playVideo(hoveredDevice)">
             <i class="bi bi-play-circle me-1"></i> 实时视频
          </button>
       </div>
    </div>
  </div>

  <DeviceDataModal 
    :visible="showDataModal" 
    :device="currentDataDevice" 
    :products="products" 
    @close="showDataModal = false" 
  />
  <!-- Dynamic Extension Modals -->
  <component 
    v-for="modal in activeExtensionModals" 
    :key="modal.name"
    :is="modal.component"
    v-bind="modal.props"
    @close="closeExtensionModal(modal.name)"
  />
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, watch } from 'vue';
import { Graph, register, ExtensionCategory, DagreLayout } from '@antv/g6';

register(ExtensionCategory.LAYOUT, 'dagre', DagreLayout);

import axios from 'axios';
import { useI18n } from 'vue-i18n';
import { usePlugins, loadPlugins } from '../plugins/registry';
import DeviceDataModal from '../components/device/DeviceDataModal.vue';

const { t, locale } = useI18n();
const { getPluginManifest, extensions } = usePlugins();
const container = ref(null);
const loading = ref(false);
let graph = null;
const currentTheme = ref('dark');

const extensionDeviceActions = computed(() => extensions.value?.deviceActions || []);
const activeExtensionModals = ref([]);

const openExtensionModal = (name, component, props) => {
  const existing = activeExtensionModals.value.find(m => m.name === name);
  if (!existing) {
    activeExtensionModals.value.push({ name, component, props });
  } else {
    existing.props = props;
  }
};

const closeExtensionModal = (name) => {
  activeExtensionModals.value = activeExtensionModals.value.filter(m => m.name !== name);
};

const playVideo = (device) => {
  const action = extensionDeviceActions.value.find(a => a.name === 'gb28181-player');
  if (action && action.action) {
      const fullDevice = devicesList.value.find(d => d.code === device.code);
      if (fullDevice) {
          action.action(fullDevice, (eventName, payload) => {
              if (eventName === 'open-modal') {
                  openExtensionModal(payload.name, action.component, { device: payload.device });
              }
          });
      }
  }
};

// Tooltip State
const tooltipVisible = ref(false);
const tooltipX = ref(0);
const tooltipY = ref(0);
const hoveredDevice = ref(null);
const loadingTooltip = ref(false);
const hoveredDeviceData = ref({});
const products = ref([]); // Cache products for TSL
const plugins = ref([]); // Cache plugins for topology
let tooltipTimer = null;
let hideTooltipTimer = null;

const onTooltipEnter = () => {
    if (hideTooltipTimer) {
        clearTimeout(hideTooltipTimer);
        hideTooltipTimer = null;
    }
};

const onTooltipLeave = () => {
    hideTooltipTimer = setTimeout(() => {
        if (tooltipTimer) {
            clearInterval(tooltipTimer);
            tooltipTimer = null;
        }
        tooltipVisible.value = false;
        hoveredDevice.value = null;
    }, 300);
};

const themeColors = computed(() => {
  const isDark = currentTheme.value === 'dark';
  return {
    nodeFill: isDark ? '#18181b' : '#fff', // Zinc 900
    nodeStroke: isDark ? '#3f3f46' : '#C0C0C0', // Zinc 700
    text: isDark ? '#f4f4f5' : '#000', // Zinc 100
    edge: isDark ? '#71717a' : '#A3B1BF', // Zinc 500
    tooltipBg: isDark ? '#27272a' : '#fff', // Zinc 800
    tooltipText: isDark ? '#f4f4f5' : '#212529', // Zinc 100
    platformFill: isDark ? '#1e3a8a' : '#e0f2fe', // Blue 900/100
    platformStroke: '#3b82f6', // Blue 500
  };
});

const tooltipDataList = computed(() => {
    if (!hoveredDevice.value) return [];
    
    // Find product config
    const product = products.value.find(p => p.code === hoveredDevice.value.product_code);
    let tslMap = {};
    if (product && product.config) {
        try {
            const prodConfig = JSON.parse(product.config);
            (prodConfig.tsl?.properties || []).forEach(p => {
                tslMap[p.identifier] = p;
            });
        } catch(e) { /* ignore */ }
    }

    // Get Point Configs
    let pointConfigs = {};
    try {
        const config = hoveredDevice.value.config ? JSON.parse(hoveredDevice.value.config) : {};
        if (config.points) {
             if (Array.isArray(config.points)) {
                config.points.forEach(p => { if (p.name) pointConfigs[p.name] = p; });
             } else {
                pointConfigs = config.points;
             }
        }
    } catch(e) { /* ignore */ }

    // Combine
    const keys = new Set([
        ...Object.keys(pointConfigs || {}),
        ...Object.keys(hoveredDeviceData.value || {})
    ]);

    const list = [];
    keys.forEach(key => {
        const tslProp = tslMap[key];
        list.push({
            key: key,
            name: tslProp ? tslProp.name : key,
            unit: tslProp && tslProp.dataType && tslProp.dataType.specs ? tslProp.dataType.specs.unit : '',
            value: hoveredDeviceData.value[key] !== undefined ? hoveredDeviceData.value[key] : '-',
        });
    });
    return list.sort((a, b) => a.key.localeCompare(b.key));
});

const updateTheme = () => {
  const appDiv = document.querySelector('[data-bs-theme]');
  if (appDiv) {
    currentTheme.value = appDiv.getAttribute('data-bs-theme') || 'dark';
  }
};

let themeObserver = null;

const devicesList = ref([]);
const showDataModal = ref(false);
const currentDataDevice = ref(null);

const fetchData = async () => {
  try {
    const res = await axios.get('/api/devices');
    if (res.data.code === 0) {
      devicesList.value = res.data.data || [];
      return devicesList.value;
    }
    devicesList.value = [];
    return [];
  } catch (e) {
    console.error(e);
    devicesList.value = [];
    return [];
  }
};

const fetchProducts = async () => {
    try {
        const res = await axios.get('/api/products');
        if (res.data.code === 0) {
            products.value = res.data.data || [];
        }
    } catch (e) { console.error(e); }
};

const fetchPlugins = async () => {
    try {
        const res = await axios.get('/api/plugins');
        if (res.data.code === 0) {
            plugins.value = res.data.data || [];
        }
    } catch (e) { console.error(e); }
};

const fetchPluginStatus = async (pluginName) => {
    try {
        // Try to fetch specific status (convention: /api/extension/{name}/status)
        const res = await axios.get(`/api/extension/${pluginName}/status`, { timeout: 1000 });
        if (res.status === 200 && res.data && res.data.status) {
            return res.data.status; // e.g., "connected", "disconnected"
        }
    } catch (e) {
        // Ignore errors as not all plugins support this
    }
    return null;
};


const fetchDeviceData = async (deviceCode) => {
    if (!deviceCode) return;
    loadingTooltip.value = true;
    try {
        const res = await axios.get(`/api/devices/${deviceCode}/data`);
        if (res.data.code === 0) {
            hoveredDeviceData.value = res.data.data || {};
        } else {
            hoveredDeviceData.value = {};
        }
    } catch (e) {
        console.error(e);
        hoveredDeviceData.value = {};
    } finally {
        loadingTooltip.value = false;
    }
};

const buildGraphData = (devices, plugins) => {
    const nodes = [];
    const edges = [];
    const colors = themeColors.value;

    // 1. Gateway Node (Center/Root)
    const gatewayId = 'root';
    
    // Determine the root node type based on cascade plugin
    let rootLabel = 'Noyo边缘网关';
    let rootType = 'gateway';
    
    if (plugins && plugins.length > 0) {
        const cascadePlugin = plugins.find(p => p.name === 'cascade');
        if (cascadePlugin && cascadePlugin.status === 'running') {
            const modeField = cascadePlugin.schema?.fields?.find(f => f.name === 'mode');
            if (modeField && modeField.value === 'platform') {
                rootLabel = 'Noyo平台';
                rootType = 'platform';
            }
        }
    }

    nodes.push({
        id: gatewayId,
        data: {
            label: rootLabel,
            type: rootType,
            status: 'online',
            isRoot: true
        },
        style: {
            fill: '#1890ff',
            stroke: '#1890ff',
            labelText: rootLabel,
            labelFill: '#fff',
        }
    });

    // 2. Platform Plugins (Northbound - Upstream)
    // Edges: Platform -> Gateway
    if (plugins && plugins.length > 0) {
        // If cascade is in gateway mode, manually add the Noyo platform node
        const cascadePlugin = plugins.find(p => p.name === 'cascade');
        if (cascadePlugin && cascadePlugin.status === 'running') {
            const modeField = cascadePlugin.schema?.fields?.find(f => f.name === 'mode');
            if (!modeField || modeField.value !== 'platform') {
                // It is in gateway mode
                nodes.push({
                    id: 'platform-noyo',
                    data: {
                        label: 'Noyo平台',
                        type: 'platform',
                        status: cascadePlugin.detailedStatus || cascadePlugin.status || 'connected',
                        protocol: 'MQTT',
                        pluginName: 'cascade'
                    }
                });

                edges.push({
                    source: 'platform-noyo',
                    target: gatewayId,
                    data: {
                        type: 'platform-link',
                        protocol: 'MQTT'
                    }
                });
            }
        }

        plugins.forEach(p => {
            if (p.category !== 'platform' || p.status !== 'running') return;

            // Filter out specific plugins from the topology graph display
            const pTitle = p.title?.['zh'] || p.title?.['en'] || p.name;
            if (['ai_copilot', 'ai_predict', 'cascade'].includes(p.name) || 
                pTitle.includes('AI智能助手') || 
                pTitle.includes('AI设备守护') || 
                pTitle.includes('级联插件')) {
                return;
            }

            let localizedTitle = p.title?.[locale.value] || p.title?.['en'] || p.name;
            // Determine Status Color
            const status = p.detailedStatus || p.status || 'disconnected';
            const displayStatus = (status === 'connected' || status === 'running') ? 'connected' : 'disconnected';
            
            // Determine Protocol from Manifest
            let protocol = 'HTTP';
            const manifest = getPluginManifest(p.name);
            if (manifest && manifest.topology && manifest.topology.protocol) {
               protocol = manifest.topology.protocol;
            }
            
            if (p.name === 'cascade') {
                localizedTitle = 'Noyo平台';
                protocol = 'MQTT';
            }
            
            nodes.push({
                id: `plugin-${p.name}`,
                data: {
                    label: localizedTitle,
                    type: 'platform',
                    status: displayStatus,
                    protocol: protocol,
                    pluginName: p.name
                }
            });

            edges.push({
                source: `plugin-${p.name}`,
                target: gatewayId,
                data: {
                    type: 'platform-link',
                    protocol: protocol
                }
            });
        });
    }

    // 3. Devices (Southbound - Downstream)
    // Edges: Gateway -> Device (or Device -> SubDevice)
    
    // First, map all devices
    const deviceMap = {};
    if (devices) {
        devices.forEach(d => {
            deviceMap[d.code] = d;
        });
    }

    // Create product protocol map
    const productProtocolMap = {};
    const productDriverProtocols = {};
    if (products.value && products.value.length > 0) {
        products.value.forEach(p => {
            productProtocolMap[p.code] = p.protocol_name;
            if (p.protocol_name && p.protocol_name.toLowerCase() === 'script' && p.config) {
                try {
                    const cfg = typeof p.config === 'string' ? JSON.parse(p.config) : p.config;
                    if (cfg.script) {
                        const script = cfg.script;
                        const cleanScript = script.replace(/--\[\[[\s\S]*?\]\]/g, '').replace(/--.*$/gm, '');
                        const protocols = new Set();
                        if (cleanScript.includes('http.get') || cleanScript.includes('http.post') || cleanScript.includes('http.request')) protocols.add('HTTP客户端');
                        if (/clients\.http\s*=/.test(cleanScript) || /clients\[['"]http['"]\]\s*=/.test(cleanScript)) protocols.add('HTTP客户端');
                        if (cleanScript.includes('listeners.http') || /servers\.http\s*=/.test(cleanScript) || /servers\[['"]http['"]\]\s*=/.test(cleanScript)) protocols.add('HTTP服务端');
                        if (cleanScript.includes('net.tcp_request') || cleanScript.includes('net.dial_tcp')) protocols.add('TCP客户端');
                        if (/clients\.tcp\s*=/.test(cleanScript) || /clients\[['"]tcp['"]\]\s*=/.test(cleanScript)) protocols.add('TCP客户端');
                        if (cleanScript.includes('listeners.tcp') || /servers\.tcp\s*=/.test(cleanScript) || /servers\[['"]tcp['"]\]\s*=/.test(cleanScript)) protocols.add('TCP服务端');
                        if (cleanScript.includes('net.udp_request') || cleanScript.includes('net.dial_udp')) protocols.add('UDP客户端');
                        if (cleanScript.includes('listeners.udp') || /servers\.udp\s*=/.test(cleanScript) || /servers\[['"]udp['"]\]\s*=/.test(cleanScript)) protocols.add('UDP服务端');
                        if (cleanScript.includes('listeners.mqtt') || cleanScript.includes('mqtt.client') || /clients\.mqtt\s*=/.test(cleanScript) || /clients\[['"]mqtt['"]\]\s*=/.test(cleanScript)) protocols.add('MQTT客户端');
                        
                        if (protocols.size > 0) {
                            productDriverProtocols[p.code] = Array.from(protocols).join(' / ');
                        }
                    }
                } catch(e){}
            }
        });
    }

    if (devices) {
        devices.forEach(d => {
            let protocol = productProtocolMap[d.product_code] || d.protocol || '';
            if (protocol && protocol.toLowerCase() === 'script' && productDriverProtocols[d.product_code]) {
                protocol = productDriverProtocols[d.product_code];
            }
            
            let label = d.name || d.code;
            if (d.product_code === 'noyo-gw') {
                label = 'Noyo边缘网关';
                protocol = 'MQTT';
            }

            const isOnline = d.online;
            const isCamera = protocol === 'gb28181' || d.protocol === 'gb28181' || d._protocol === 'gb28181';
            nodes.push({
                id: d.code,
                data: {
                    label: label,
                    type: 'device',
                    status: d.enabled ? (d.online ? 'online' : (d.status === 'running' ? 'offline' : 'stopped')) : 'disabled',
                    protocol: protocol,
                    product_code: d.product_code,
                    isCamera: isCamera,
                    code: d.code,
                    name: d.name,
                    online: d.online
                }
            });

            // Determine Parent
            let parentId = gatewayId;
            if (d.parent_code && deviceMap[d.parent_code]) {
                parentId = d.parent_code;
            }

            edges.push({
                source: parentId,
                target: d.code,
                data: {
                    type: 'device-link',
                    protocol: protocol
                }
            });
        });
    }

    return { nodes, edges };
};

const initGraph = async () => {
  if (!container.value) return;
  
  loading.value = true;
  // Ensure products and plugins are loaded
  const promises = [fetchData(), loadPlugins()]; // Add loadPlugins here
  if (products.value.length === 0) promises.push(fetchProducts());
  if (plugins.value.length === 0) promises.push(fetchPlugins());
  
  const [devices] = await Promise.all(promises);

  // Enrich plugins with specific status
  if (plugins.value.length > 0) {
      const statusPromises = plugins.value.map(async p => {
          if (p.category === 'platform' && p.status === 'running') {
             const status = await fetchPluginStatus(p.name);
             if (status) {
                 p.detailedStatus = status;
             }
          }
      });
      await Promise.all(statusPromises);
  }

  const data = buildGraphData(devices, plugins.value);

  // Destroy existing instance if any
  if (graph) graph.destroy();

  const width = container.value.clientWidth;
  const height = container.value.clientHeight || 600;

  graph = new Graph({
    container: container.value,
    width,
    height,
    data,
    layout: {
      type: 'dagre',
      rankdir: 'TB', // Top to Bottom
      nodesep: 80,
      ranksep: 150,
      controlPoints: true,
    },
    node: {
        type: (d) => {
            const isRoot = d.data?.isRoot;
            const type = d.data?.type;
            const label = d.data?.label || '';
            const productCode = d.data?.product_code || '';
            const isCamera = d.data?.isCamera;
            
            if (isRoot || type === 'platform' || isCamera) return 'circle';
            if (productCode === 'noyo-gw' || label.includes('网关') || label.toLowerCase().includes('gateway')) return 'circle';
            if (label.includes('子系统') || label.toLowerCase().includes('subsystem')) return 'circle';
            
            return 'rect';
        },
        style: (d) => {
            let color = '#A0A0A0'; // Default disabled
            const status = d.data?.status;
            const type = d.data?.type;
            const isRoot = d.data?.isRoot;
            const label = d.data?.label || '';
            const protocol = d.data?.protocol || '';
            const productCode = d.data?.product_code || '';
            const isCamera = d.data?.isCamera;
            const colors = themeColors.value;

            // Status Colors
            if (status === 'online' || status === 'running' || status === 'connected') color = '#64BB5C';
            else if (status === 'offline' || status === 'stopped' || status === 'disconnected') color = '#6c757d';
            else if (status === 'disabled') color = '#A0A0A0';
            else if (status === 'alarm') color = '#E84026';
            
            let fill = colors.nodeFill;
            let labelFill = colors.text;
            let stroke = color;

            const isGateway = productCode === 'noyo-gw' || label.includes('网关') || label.toLowerCase().includes('gateway');
            const isSubsystem = label.includes('子系统') || label.toLowerCase().includes('subsystem');

            if (isRoot || type === 'platform' || isGateway || isSubsystem || isCamera) {
                if (isRoot) {
                    fill = '#1890ff';
                    stroke = '#1890ff';
                    labelFill = colors.text;
                } else if (type === 'platform') {
                    fill = colors.platformFill;
                    stroke = color; 
                    labelFill = colors.text;
                }

                // Determine icon
                let icon = '\uF77D'; // bi-plugin for other platforms
                if (isRoot) {
                    if (type === 'platform') {
                        icon = '\uF29E'; // bi-cloud-fill for Noyo platform
                    } else {
                        icon = '\uF40D'; // bi-hdd-network for Noyo edge gateway
                    }
                } else if (type === 'platform') {
                    if (d.data?.pluginName === 'cascade') {
                        icon = '\uF29E'; // bi-cloud-fill for Noyo platform
                    }
                } else if (isCamera) {
                    icon = '\uF21F'; // bi-camera-video
                } else if (isSubsystem) {
                    icon = '\uF2EE'; // bi-diagram-3 for subsystem
                } else if (isGateway && protocol.toLowerCase().includes('modbus')) {
                    icon = '\uF4F7'; // bi-plug for modbus
                } else if (isGateway) {
                    icon = '\uF40D'; // bi-hdd-network for other gateways
                }

                const maxLength = 10;
                let displayLabel = label || d.id;
                if (displayLabel.length > maxLength) {
                    displayLabel = displayLabel.substring(0, maxLength) + '...';
                }

                return {
                    size: [56, 56],
                    labelText: displayLabel,
                    labelFill: labelFill,
                    fill: fill,
                    stroke: stroke,
                    lineWidth: 2,
                    cursor: 'pointer',
                    labelPlacement: 'bottom',
                    labelOffsetY: 8,
                    iconText: icon,
                    iconFontFamily: 'bootstrap-icons',
                    iconFill: isRoot ? '#fff' : (type === 'platform' ? colors.platformStroke : stroke),
                    iconFontSize: 28,
                    ports: [{ placement: 'top' }, { placement: 'bottom' }]
                };
            }

            const maxLength = 15;
            let displayLabel = label || d.id;
            if (displayLabel.length > maxLength) {
                displayLabel = displayLabel.substring(0, maxLength) + '...';
            }

            return {
                size: [160, 40],
                labelText: displayLabel,
                labelFill: labelFill,
                fill: fill,
                stroke: stroke,
                lineWidth: 2,
                radius: 6,
                cursor: 'pointer',
                labelPlacement: 'center',
                ports: [{ placement: 'top' }, { placement: 'bottom' }]
            };
        }
    },
    edge: {
        type: 'cubic-vertical', 
        style: (d) => {
            const colors = themeColors.value;
            const isPlatform = d.data?.type === 'platform-link';
            return {
                startArrow: isPlatform,
                endArrow: true,
                stroke: isPlatform ? colors.platformStroke : colors.edge,
                labelText: d.data?.protocol || '',
                labelFill: isPlatform ? colors.platformStroke : colors.text,
                labelBackground: true,
                labelBackgroundFill: colors.tooltipBg,
                labelFontSize: 10,
                labelBackgroundRadius: 4,
                labelPadding: [2, 4],
            };
        },
    },
    behaviors: ['drag-canvas', 'zoom-canvas', 'drag-element'],
    animation: true
  });

  await graph.render();
  graph.fitView();
  
  loading.value = false;

  // Re-attach event listeners
  graph.on('node:pointerenter', (e) => {
     const nodeId = e.target.id;
     const nodeData = nodeId ? graph.getNodeData(nodeId) : null;
     
     if (nodeData && !nodeData.data.isRoot) {
         if (hideTooltipTimer) {
             clearTimeout(hideTooltipTimer);
             hideTooltipTimer = null;
         }
         if (tooltipTimer) clearInterval(tooltipTimer);
         
         hoveredDevice.value = nodeData.data;
         hoveredDeviceData.value = {}; 
         tooltipVisible.value = true;
         tooltipX.value = e.client.x + 15;
         tooltipY.value = e.client.y + 15;
         
         fetchDeviceData(nodeId);
         tooltipTimer = setInterval(() => fetchDeviceData(nodeId), 2000);
     }
  });

  graph.on('node:pointerleave', () => {
     hideTooltipTimer = setTimeout(() => {
         if (tooltipTimer) {
             clearInterval(tooltipTimer);
             tooltipTimer = null;
         }
         tooltipVisible.value = false;
         hoveredDevice.value = null;
     }, 300);
  });

  graph.on('node:click', (e) => {
     const nodeId = e.target.id;
     if (!nodeId) return;
     const nodeData = graph.getNodeData(nodeId);
     
     if (nodeData && nodeData.data && !nodeData.data.isRoot && nodeData.data.type !== 'platform') {
         const device = devicesList.value.find(d => d.code === nodeId);
         if (device) {
             currentDataDevice.value = device;
             showDataModal.value = true;
         }
     }
  });

  graph.on('node:drag', () => { 
      tooltipVisible.value = false; 
      if (tooltipTimer) clearInterval(tooltipTimer);
  });
  graph.on('canvas:drag', () => { 
      tooltipVisible.value = false; 
      if (tooltipTimer) clearInterval(tooltipTimer);
  });
  graph.on('wheel', () => { 
      tooltipVisible.value = false; 
      if (tooltipTimer) clearInterval(tooltipTimer);
  });
};

const fitView = () => {
  if (graph) graph.fitView();
};

const refresh = () => {
  initGraph();
};

const handleResize = () => {
    if (graph && container.value) {
        graph.setSize(container.value.clientWidth, container.value.clientHeight);
        graph.fitView();
    }
};

onMounted(() => {
  // Delay slightly to ensure container is ready and has dimensions
  setTimeout(() => {
    initGraph();
  }, 100);
  window.addEventListener('resize', handleResize);

  updateTheme();
  fetchProducts(); // Load products for TSL lookup
  const appDiv = document.querySelector('[data-bs-theme]');
  if (appDiv) {
      themeObserver = new MutationObserver(updateTheme);
      themeObserver.observe(appDiv, { attributes: true, attributeFilter: ['data-bs-theme'] });
  }
});

watch(currentTheme, () => {
    initGraph();
});

onUnmounted(() => {
  if (graph) graph.destroy();
  if (themeObserver) themeObserver.disconnect();
  window.removeEventListener('resize', handleResize);
});
</script>

<style scoped>
#mountNode {
  background: var(--bs-body-bg);
}
</style>
