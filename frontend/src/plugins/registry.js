
import { ref, markRaw } from 'vue';

// Global registry state
const plugins = ref(new Map());
const isLoaded = ref(false);

// Extension registry
const extensions = ref({
  routes: [],
  menus: [],
  deviceActions: []
});

/**
 * Auto-load all plugins using Vite's glob import
 */
export async function loadPlugins() {
  if (isLoaded.value) return;

  const modules = import.meta.glob('./**/*/index.js', { eager: true });

  for (const path in modules) {
    const mod = modules[path];
    if (mod.default && mod.default.name) {
      const pluginManifest = markRaw(mod.default);
      plugins.value.set(mod.default.name, pluginManifest);
      
      // Register extensions
      if (pluginManifest.routes) {
        extensions.value.routes.push(...pluginManifest.routes);
      }
      if (pluginManifest.menus) {
        extensions.value.menus.push(...pluginManifest.menus);
      }
      if (pluginManifest.deviceActions) {
        extensions.value.deviceActions.push(...pluginManifest.deviceActions);
      }
    }
  }

  isLoaded.value = true;
  console.log(`[PluginRegistry] Loaded ${plugins.value.size} plugins`, plugins.value.keys());
}

/**
 * Get plugin manifest by name (case-insensitive)
 * @param {string} name 
 * @returns {Object|null}
 */
export function getPluginManifest(name) {
  if (!name) return null;
  // Try direct match
  if (plugins.value.has(name)) return plugins.value.get(name);
  
  // Try case-insensitive match
  const lowerName = name.toLowerCase();
  for (const [key, val] of plugins.value.entries()) {
    if (key.toLowerCase() === lowerName) return val;
  }
  return null;
}

export function usePlugins() {
  if (!isLoaded.value) {
    loadPlugins(); // Trigger load if not ready
  }
  return {
    getPluginManifest,
    plugins,
    extensions
  };
}
