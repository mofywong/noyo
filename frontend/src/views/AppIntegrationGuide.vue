<template>
  <div class="container-fluid py-4">
    <div class="d-flex align-items-center mb-4">
      <button class="btn btn-link p-0 me-3 text-secondary" @click="$router.back()">
        <i class="bi bi-arrow-left fs-4"></i>
      </button>
      <h2 class="h4 mb-0 fw-bold text-primary border-start border-primary border-4 ps-2">{{ t('app_guide_title') }}</h2>
    </div>

    <div class="card shadow-sm">
      <div class="card-body p-4">
        <h5 class="card-title text-primary mb-3">{{ t('app_guide_intro_title') }}</h5>
        <p class="card-text text-muted mb-4">{{ t('app_guide_intro') }}</p>

        <div class="alert alert-primary d-flex align-items-start gap-3 mb-4">
          <i class="bi bi-shield-lock-fill fs-4"></i>
          <div>
            <div class="fw-semibold">{{ t('app_auth_method_title') }}</div>
            <div>{{ t('app_auth_method_current') }}</div>
          </div>
        </div>

        <div class="alert alert-secondary d-flex align-items-start gap-3 mb-4">
          <i class="bi bi-diagram-3-fill fs-4"></i>
          <div>
            <div class="fw-semibold">{{ t('app_guide_access_scope_title') }}</div>
            <div>{{ t('app_guide_access_scope') }}</div>
          </div>
        </div>

        <section class="guide-section">
          <h6 class="fw-bold"><i class="bi bi-1-circle text-primary me-2"></i>{{ t('app_guide_token_step_title') }}</h6>
          <p class="text-muted">{{ t('app_guide_token_step_desc') }}</p>
          <div class="endpoint-row">
            <span>{{ t('app_guide_endpoint') }}</span>
            <code>POST /api/auth/app-token</code>
          </div>
          <h6 class="small fw-bold text-secondary mt-3">{{ t('app_guide_request_body') }}</h6>
          <div class="table-responsive">
            <table class="table table-sm align-middle">
              <thead>
                <tr>
                  <th>{{ t('app_guide_param_name') }}</th>
                  <th>{{ t('app_guide_required') }}</th>
                  <th>{{ t('app_guide_description') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr>
                  <td><code>app_id</code></td>
                  <td>{{ t('app_guide_yes') }}</td>
                  <td>{{ t('app_guide_param_app_id') }}</td>
                </tr>
                <tr>
                  <td><code>app_key</code></td>
                  <td>{{ t('app_guide_yes') }}</td>
                  <td>{{ t('app_guide_param_app_key') }}</td>
                </tr>
              </tbody>
            </table>
          </div>
          <div class="code-block">
            <pre class="mb-0"><code>curl -X POST "https://your-iot-domain.com/api/auth/app-token" \
  -H "Content-Type: application/json" \
  -d '{"app_id":"your-app-id","app_key":"your-app-key"}'</code></pre>
          </div>
          <h6 class="small fw-bold text-secondary mt-3">{{ t('app_guide_response_fields') }}</h6>
          <div class="code-block">
            <pre class="mb-0"><code>{
  "code": 0,
  "data": {
    "access_token": "eyJ...",
    "refresh_token": "eyJ...",
    "token_type": "Bearer",
    "expires_in": 7200,
    "refresh_expires_in": 604800
  }
}</code></pre>
          </div>
        </section>

        <section class="guide-section">
          <h6 class="fw-bold"><i class="bi bi-2-circle text-primary me-2"></i>{{ t('app_guide_device_step_title') }}</h6>
          <p class="text-muted">{{ t('app_guide_device_step_desc') }}</p>
          <div class="endpoint-row">
            <span>{{ t('app_guide_endpoint') }}</span>
            <code>GET /api/devices?page=1&amp;pageSize=10</code>
          </div>
          <h6 class="small fw-bold text-secondary mt-3">{{ t('app_guide_headers') }}</h6>
          <div class="table-responsive">
            <table class="table table-sm align-middle">
              <thead>
                <tr>
                  <th>{{ t('app_guide_param_name') }}</th>
                  <th>{{ t('app_guide_required') }}</th>
                  <th>{{ t('app_guide_description') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr>
                  <td><code>Authorization</code></td>
                  <td>{{ t('app_guide_yes') }}</td>
                  <td>{{ t('app_guide_header_authorization') }}</td>
                </tr>
                <tr>
                  <td><code>X-Current-Project-ID</code></td>
                  <td>{{ t('app_guide_optional') }}</td>
                  <td>{{ t('app_guide_header_project') }}</td>
                </tr>
              </tbody>
            </table>
          </div>
          <h6 class="small fw-bold text-secondary mt-3">{{ t('app_guide_query_params') }}</h6>
          <div class="table-responsive">
            <table class="table table-sm align-middle">
              <thead>
                <tr>
                  <th>{{ t('app_guide_param_name') }}</th>
                  <th>{{ t('app_guide_required') }}</th>
                  <th>{{ t('app_guide_description') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr>
                  <td><code>page</code></td>
                  <td>{{ t('app_guide_optional') }}</td>
                  <td>{{ t('app_guide_param_page') }}</td>
                </tr>
                <tr>
                  <td><code>pageSize</code></td>
                  <td>{{ t('app_guide_optional') }}</td>
                  <td>{{ t('app_guide_param_page_size') }}</td>
                </tr>
              </tbody>
            </table>
          </div>
          <div class="code-block">
            <pre class="mb-0"><code>curl -X GET "https://your-iot-domain.com/api/devices?page=1&amp;pageSize=10" \
  -H "Authorization: Bearer eyJ..." \
  -H "X-Current-Project-ID: 1"</code></pre>
          </div>
        </section>

        <section class="guide-section">
          <h6 class="fw-bold"><i class="bi bi-3-circle text-primary me-2"></i>{{ t('app_guide_refresh_step_title') }}</h6>
          <p class="text-muted">{{ t('app_guide_refresh_step_desc') }}</p>
          <div class="endpoint-row">
            <span>{{ t('app_guide_endpoint') }}</span>
            <code>POST /api/auth/app-refresh</code>
          </div>
          <h6 class="small fw-bold text-secondary mt-3">{{ t('app_guide_request_body') }}</h6>
          <div class="table-responsive">
            <table class="table table-sm align-middle">
              <thead>
                <tr>
                  <th>{{ t('app_guide_param_name') }}</th>
                  <th>{{ t('app_guide_required') }}</th>
                  <th>{{ t('app_guide_description') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr>
                  <td><code>refresh_token</code></td>
                  <td>{{ t('app_guide_yes') }}</td>
                  <td>{{ t('app_guide_param_refresh_token') }}</td>
                </tr>
              </tbody>
            </table>
          </div>
          <div class="code-block">
            <pre class="mb-0"><code>curl -X POST "https://your-iot-domain.com/api/auth/app-refresh" \
  -H "Content-Type: application/json" \
  -d '{"refresh_token":"eyJ..."}'</code></pre>
          </div>
        </section>

        <h6 class="mt-4 fw-bold"><i class="bi bi-lock text-primary me-2"></i>{{ t('app_guide_security_title') }}</h6>
        <ul class="text-muted">
          <li class="mb-2">{{ t('app_guide_secret') }}</li>
          <li class="mb-2">{{ t('app_guide_reset') }}</li>
          <li>{{ t('app_guide_rate_limit') }}</li>
        </ul>
      </div>
    </div>
  </div>
</template>

<script setup>
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
</script>

<style scoped>
.guide-section {
  border-top: 1px solid #e5e7eb;
  padding-top: 1.25rem;
  margin-top: 1.25rem;
}

.endpoint-row {
  align-items: center;
  background: #f8fafc;
  border: 1px solid #e5e7eb;
  border-radius: 6px;
  display: flex;
  gap: 0.75rem;
  justify-content: space-between;
  padding: 0.75rem 1rem;
}

.code-block {
  background: #111827;
  border-radius: 6px;
  color: #f9fafb;
  margin-top: 0.75rem;
  overflow-x: auto;
  padding: 1rem;
}

.code-block code {
  color: inherit;
}

@media (max-width: 576px) {
  .endpoint-row {
    align-items: flex-start;
    flex-direction: column;
  }
}
</style>
