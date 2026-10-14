// Shared by AI recognition, model management, and the compiled model provider.
import catalog from '../../../backend/core/aiassets/recognition_catalog.json' with { type: 'json' };

export const RECOGNITION_CATEGORIES = catalog.categories;
export const RECOGNITION_CATALOG = catalog.items;
export const SCENE_CATEGORIES = [
  { id: 'all', icon: 'bi-grid', name: { zh: '全部能力', en: 'All capabilities' } },
  ...RECOGNITION_CATEGORIES.filter(category => category.id !== 'basic'),
];
export const SCENE_CATALOG = RECOGNITION_CATALOG.filter(item => item.rule);
