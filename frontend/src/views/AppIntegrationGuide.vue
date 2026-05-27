<template>
  <div class="container-fluid py-4">
    <div class="d-flex align-items-center mb-4">
      <button class="btn btn-link p-0 me-3 text-secondary" @click="$router.back()">
        <i class="bi bi-arrow-left fs-4"></i>
      </button>
      <h2 class="h4 mb-0 fw-bold text-primary border-start border-primary border-4 ps-2">应用接入指南</h2>
    </div>

    <div class="card shadow-sm">
      <div class="card-body p-4">
        <h5 class="card-title text-primary mb-4">如何使用 AppID 和 AppKey 接入 IOT 平台</h5>
        
        <p class="card-text">
          欢迎使用 IOT 平台提供的开放 API。通过创建应用，您将获得一组唯一的 <code>AppID</code> 和 <code>AppKey</code>，用于识别和验证您的请求。
        </p>

        <h6 class="mt-4 fw-bold"><i class="bi bi-1-circle text-primary me-2"></i>身份认证机制</h6>
        <p class="text-muted mb-2">
          在调用平台 API 时，您需要在 HTTP 请求头 (Headers) 中传递身份认证信息：
        </p>
        <ul class="list-group list-group-flush mb-4">
          <li class="list-group-item bg-light border-0 mb-1 rounded"><code>X-App-Id</code>: 您分配到的 AppID</li>
          <li class="list-group-item bg-light border-0 rounded"><code>X-App-Key</code>: 您分配到的 AppKey</li>
        </ul>

        <h6 class="mt-4 fw-bold"><i class="bi bi-2-circle text-primary me-2"></i>请求示例 (cURL)</h6>
        <div class="bg-dark text-light p-3 rounded mb-4">
          <pre class="mb-0"><code>curl -X GET "https://your-iot-domain.com/api/devices" \
  -H "X-App-Id: 您的AppID" \
  -H "X-App-Key: 您的AppKey"</code></pre>
        </div>

        <h6 class="mt-4 fw-bold"><i class="bi bi-3-circle text-primary me-2"></i>安全建议</h6>
        <ul class="text-muted">
          <li class="mb-2"><strong>保密性:</strong> <code>AppKey</code> 类似于密码，请务必妥善保管，切勿在客户端代码（如网页 JS）或公开仓库中硬编码暴露。</li>
          <li class="mb-2"><strong>重置密钥:</strong> 如果您怀疑 <code>AppKey</code> 已泄露，请立即在应用管理页面点击“重置密钥”生成新的密钥，旧密钥将自动失效。</li>
          <li><strong>合理限流:</strong> 平台支持对应用进行 API 调用频率限制。如遇到 429 错误 (Too Many Requests)，请检查您的调用频率或联系管理员调整限流额度。</li>
        </ul>
        
        <div class="alert alert-info mt-4 d-flex align-items-center">
          <i class="bi bi-info-circle-fill fs-4 me-3"></i> 
          <div>如需查看完整的 API 接口文档及参数说明，请访问我们的开发者中心或联系技术支持团队。</div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
</script>
