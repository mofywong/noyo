package assistant

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"noyo/core/llmlog"

	einoopenai "github.com/cloudwego/eino-ext/components/model/openai"
	einomodel "github.com/cloudwego/eino/components/model"
)

type ProviderConfig struct {
	BaseURL  string
	APIKey   string
	Model    string
	Timeout  time.Duration
	Metadata llmlog.Metadata
	// IOC opts into explicit reasoning; shared chat keeps provider defaults.
	StructuredReasoning bool
}

func newOpenAICompatibleChatModel(ctx context.Context, cfg ProviderConfig) (einomodel.ToolCallingChatModel, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, fmt.Errorf("assistant provider api key is required")
	}
	if strings.TrimSpace(cfg.Model) == "" {
		return nil, fmt.Errorf("assistant provider model is required")
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}
	metadata := cfg.Metadata
	if scope, ok := ScopeFromContext(ctx); ok {
		metadata = llmlog.Merge(metadata, llmlog.Metadata{TenantID: scope.TenantID, ProjectID: scope.ProjectID, UserID: scope.UserID})
	}
	metadata = llmlog.Merge(metadata, llmlog.FromContext(ctx))
	if metadata.Source == "" {
		metadata.Source = "assistant"
	}

	config := &einoopenai.ChatModelConfig{
		APIKey:     cfg.APIKey,
		BaseURL:    cfg.BaseURL,
		Model:      cfg.Model,
		Timeout:    timeout,
		HTTPClient: &http.Client{Timeout: timeout, Transport: llmlog.NewTransport(nil, metadata)},
	}
	// Preserve reasoning quality and give the bounded workflow enough time.
	// Vendor-specific controls must never leak to other compatible models.
	if cfg.StructuredReasoning {
		switch strings.ToLower(strings.TrimSpace(cfg.Model)) {
		case "deepseek-flash", "deepseek-v4-pro":
			config.ExtraFields = map[string]interface{}{"thinking": map[string]string{"type": "enabled"}, "reasoning_effort": "high", "max_tokens": 32768}
		}
	}
	return einoopenai.NewChatModel(ctx, config)
}
