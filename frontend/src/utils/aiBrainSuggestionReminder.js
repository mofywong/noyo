export const isSuccessfulDeviceWriteResponse = (response) => {
  const method = String(response?.config?.method || '').toLowerCase();
  const url = String(response?.config?.url || '');
  return method === 'post'
    && /\/api\/devices\/[^/]+\/write(?:\?|$)/.test(url)
    && response?.data?.code === 0;
};
