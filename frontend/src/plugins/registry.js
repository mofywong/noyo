
import { ref, markRaw } from 'vue';

// Global registry state
const plugins = ref(new Map());
const isLoaded = ref(false);

/**
 * Auto-load all plugins using Vite's glob import
 */
export async function loadPlugins() {
  if (isLoaded.value) return;

  // Glob import for all index.js files in subdirectories of src/plugins
  // Eager load to have them ready, or lazy import if preferred.
  // Using eager for simplicity as we need metadata immediately for lists/topology.
  const modules = import.meta.glob('./**/*/index.js', { eager: true });

  for (const path in modules) {
    const mod = modules[path];
    if (mod.default && mod.default.name) {
      // markRaw to avoid Vue reactivity performance overhead on static component definitions
      plugins.value.set(mod.default.name, markRaw(mod.default));
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
    plugins
  };
}
