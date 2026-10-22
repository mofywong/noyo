import type { App } from 'vue';
import { createPinia, setActivePinia } from 'pinia';

const pinia = createPinia();
// A bundled chunk can be evaluated before `setupStore(app)` runs, and upstream
// resolves stores at module scope. Activating here keeps those modules valid
// regardless of how the bundler orders chunks; `app.use(pinia)` re-activates it.
setActivePinia(pinia);

export function setupStore(app: App<Element>) {
  app.use(pinia);
}

export { pinia };
