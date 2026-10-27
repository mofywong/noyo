package core

import (
	"context"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
	"gorm.io/gorm"
	"noyo/core/assistant"
	"noyo/core/llmlog"
	"noyo/core/store"
)

// screenTitleSource keeps model-generated conversation titles out of the
// assistant's own request stream in the LLM history page.
const screenTitleSource = "ioc_screen_title"

// screenConversationWhere reproduces screenConversationScope without a request:
// titles are generated after the response has been written, so there is no
// *ghttp.Request left to authenticate against.
func screenConversationWhere(tenantID, projectID, screenID, userID uint, conversationID string) *gorm.DB {
	return store.DB.Where("tenant_id = ? AND project_id = ? AND screen_id = ? AND user_id = ? AND conversation_id = ?", tenantID, projectID, screenID, userID, conversationID)
}

// Titles are derived from the request plus the summary the model just produced,
// so they describe what the thread is about instead of quoting the first 80
// characters of a multi-clause prompt. The work is detached from the request:
// a transcript must never wait for, or fail because of, a cosmetic label.
func (s *Server) screenAssistantTitleAsync(provider assistant.ProviderConfig, tenantID, projectID, screenID, userID uint, conversationID, prompt, summary, locale string) {
	if conversationID == "" || conversationID == "legacy" || strings.TrimSpace(prompt) == "" {
		return
	}
	var current store.IOCScreenConversation
	if err := screenConversationWhere(tenantID, projectID, screenID, userID, conversationID).First(&current).Error; err != nil {
		return
	}
	previous := current.Title
	go func() {
		// The request context dies with the response, so this needs its own.
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		ctx = assistant.WithScope(ctx, assistant.Scope{TenantID: tenantID, ProjectID: projectID, UserID: userID})
		ctx = llmlog.WithMetadata(ctx, llmlog.Metadata{TenantID: tenantID, ProjectID: projectID, UserID: userID, SessionID: conversationID, Source: screenTitleSource})
		runtime, err := assistant.NewRuntime(ctx, assistant.RuntimeConfig{Provider: provider})
		if err != nil {
			return
		}
		message, err := runtime.Generate(ctx, screenAssistantTitleMessages(prompt, summary, locale))
		if err != nil || message == nil {
			return
		}
		title := screenAssistantCleanTitle(message.Content)
		if title == "" {
			return
		}
		// Replace only the title this request could have written. A retry, a second
		// tab, or a rename must not be overwritten by a late completion.
		screenConversationWhere(tenantID, projectID, screenID, userID, conversationID).
			Where("title = ?", previous).
			Update("title", title)
	}()
}

func screenAssistantTitleMessages(prompt, summary, locale string) []*schema.Message {
	instruction := "为这次 IOC 大屏设计对话起一个标题。只输出标题本身：中文 6-12 个字，不要引号、句号、书名号，不要前缀或解释。"
	if locale == "en" {
		instruction = "Name this IOC screen design conversation. Reply with the title only: 3 to 6 English words, no quotes, no full stop, no prefix or explanation."
	}
	content := "需求：" + screenAssistantText(strings.TrimSpace(prompt), 800)
	if summary = strings.TrimSpace(summary); summary != "" {
		content += "\n方案：" + screenAssistantText(summary, 800)
	}
	return []*schema.Message{schema.SystemMessage(instruction), schema.UserMessage(content)}
}

const screenTitleTrim = "\"'“”‘’「」『』《》〈〉()（）[]【】 \t\r\n。，、,.;；:：!！?？-—"

var screenTitlePrefixes = []string{"标题：", "标题:", "Title:", "title:", "标题 "}

// Models occasionally ignore "title only" and answer "标题：xxx" or wrap the
// label in quotes. Both are still usable once trimmed.
func screenAssistantCleanTitle(raw string) string {
	line := raw
	if at := strings.IndexAny(line, "\r\n"); at >= 0 {
		line = line[:at]
	}
	line = strings.Trim(line, screenTitleTrim)
	for _, prefix := range screenTitlePrefixes {
		if strings.HasPrefix(line, prefix) {
			line = strings.Trim(strings.TrimPrefix(line, prefix), screenTitleTrim)
			break
		}
	}
	runes := []rune(line)
	if len(runes) > 40 {
		runes = runes[:40]
	}
	return strings.Trim(string(runes), screenTitleTrim)
}
