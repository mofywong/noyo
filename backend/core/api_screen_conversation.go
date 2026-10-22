package core

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/gogf/gf/v2/net/ghttp"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"noyo/core/store"
)

type screenConversationMessage struct {
	ID             string          `json:"id"`
	ConversationID string          `json:"conversationId,omitempty"`
	Role           string          `json:"role"`
	Content        string          `json:"content"`
	Context        json.RawMessage `json:"context,omitempty"`
	Status         string          `json:"status,omitempty"`
	Stages         []string        `json:"stages,omitempty"`
	ChangeCount    int             `json:"changeCount,omitempty"`
	TaskID         string          `json:"taskId,omitempty"`
}

type screenConversationItem struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func screenConversationItemOf(row store.IOCScreenConversation) screenConversationItem {
	return screenConversationItem{row.ConversationID, row.Title, row.CreatedAt, row.UpdatedAt}
}

func screenConversationStorageID(id string) (string, error) {
	if id == "" || id == "legacy" {
		return "", nil
	}
	if !screenAuditRequestID.MatchString(id) {
		return "", screenFail(400, "Invalid conversation / 对话无效")
	}
	return id, nil
}

func screenConversationRequire(scope *gorm.DB, id string) error {
	if id == "" {
		return nil // Existing records retain the original private conversation.
	}
	var conversation store.IOCScreenConversation
	err := scope.Where("conversation_id = ?", id).First(&conversation).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return screenFail(404, "Conversation not found / 对话不存在")
	}
	return err
}

func screenConversationOwner(r *ghttp.Request, row *store.IOCScreen, id string) store.IOCScreenConversation {
	return store.IOCScreenConversation{TenantID: row.TenantID, ProjectID: row.ProjectID, ScreenID: row.ID, UserID: requestAuthContext(r).UserID, ConversationID: id}
}

// Old message rows are not rewritten or discarded. Lazily expose their original
// thread in the new list, preserving creation time and user/screen isolation.
func screenConversationExposeLegacy(r *ghttp.Request, row *store.IOCScreen) error {
	scope := screenConversationScope(r, row)
	var first store.IOCScreenMessage
	result := scope.Where("conversation_id = ?", "").Order("id ASC").Limit(1).Find(&first)
	if result.Error != nil || result.RowsAffected == 0 {
		return result.Error
	}
	var last store.IOCScreenMessage
	if err := scope.Where("conversation_id = ?", "").Order("id DESC").First(&last).Error; err != nil {
		return err
	}
	legacy := screenConversationOwner(r, row, "legacy")
	legacy.CreatedAt, legacy.UpdatedAt = first.CreatedAt, last.UpdatedAt
	var message screenConversationMessage
	if json.Unmarshal([]byte(first.Payload), &message) == nil && message.Role == "user" {
		legacy.Title = screenAssistantText(strings.Join(strings.Fields(message.Content), " "), 80)
	}
	return store.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&legacy).Error
}

func (s *Server) handleScreenAssistantConversations(r *ghttp.Request) {
	row, err := screenLoad(r, "screen:edit")
	if err != nil {
		screenWriteError(r, err)
		return
	}
	if err = screenConversationExposeLegacy(r, row); err != nil {
		screenWriteError(r, err)
		return
	}
	query := screenConversationScope(r, row)
	if before := r.GetQuery("before").Uint(); before > 0 {
		query = query.Where("id < ?", before)
	}
	var rows []store.IOCScreenConversation
	if err = query.Order("id DESC").Limit(51).Find(&rows).Error; err != nil {
		screenWriteError(r, err)
		return
	}
	hasMore := len(rows) > 50
	if hasMore {
		rows = rows[:50]
	}
	items := make([]screenConversationItem, 0, len(rows))
	var before uint
	for _, conversation := range rows {
		items = append(items, screenConversationItemOf(conversation))
		before = conversation.ID
	}
	screenWriteData(r, map[string]interface{}{"items": items, "before": before, "hasMore": hasMore})
}

func (s *Server) handleScreenAssistantConversationCreate(r *ghttp.Request) {
	row, err := screenLoad(r, "screen:edit")
	if err != nil {
		screenWriteError(r, err)
		return
	}
	// Expose the old thread before allocating a new ID, so the newly created
	// conversation remains first in the stable creation-order pagination.
	if err = screenConversationExposeLegacy(r, row); err != nil {
		screenWriteError(r, err)
		return
	}
	var token [16]byte
	if _, err = rand.Read(token[:]); err != nil {
		screenWriteError(r, err)
		return
	}
	conversation := screenConversationOwner(r, row, hex.EncodeToString(token[:]))
	if err = store.DB.Create(&conversation).Error; err != nil {
		screenWriteError(r, err)
		return
	}
	screenWriteData(r, screenConversationItemOf(conversation))
}

func screenConversationScope(r *ghttp.Request, row *store.IOCScreen) *gorm.DB {
	return store.DB.Where("tenant_id = ? AND project_id = ? AND screen_id = ? AND user_id = ?", row.TenantID, row.ProjectID, row.ID, requestAuthContext(r).UserID).Session(&gorm.Session{})
}

// The active conversation is the only source of historical model context.
// Client-supplied transcripts cannot introduce another conversation's messages.
func screenConversationPlanHistory(scope *gorm.DB, id, prompt string) ([]screenAssistantHistoryMessage, error) {
	storageID, err := screenConversationStorageID(id)
	if err == nil {
		err = screenConversationRequire(scope, storageID)
	}
	if err != nil {
		return nil, err
	}
	var rows []store.IOCScreenMessage
	if err = scope.Where("conversation_id = ?", storageID).Order("id DESC").Limit(24).Find(&rows).Error; err != nil {
		return nil, err
	}
	history := make([]screenAssistantHistoryMessage, 0, 8)
	for i := len(rows) - 1; i >= 0; i-- {
		var message screenConversationMessage
		if json.Unmarshal([]byte(rows[i].Payload), &message) != nil || strings.TrimSpace(message.Content) == "" || (message.Role != "user" && message.Role != "assistant") {
			continue
		}
		switch message.Status {
		case "failed", "interrupted", "generating", "waiting", "canceled", "discarded":
			continue
		}
		history = append(history, screenAssistantHistoryMessage{Role: message.Role, Content: screenAssistantText(message.Content, 1000)})
	}
	// The UI persists the current user request before invoking the model.
	if n := len(history); n > 0 && history[n-1].Role == "user" && history[n-1].Content == screenAssistantText(prompt, 1000) {
		history = history[:n-1]
	}
	if len(history) > 8 {
		history = history[len(history)-8:]
	}
	return history, nil
}

func (s *Server) handleScreenAssistantHistory(r *ghttp.Request) {
	row, err := screenLoad(r, "screen:edit")
	if err != nil {
		screenWriteError(r, err)
		return
	}
	query := screenConversationScope(r, row)
	conversationID, err := screenConversationStorageID(r.GetQuery("conversationId").String())
	if err == nil {
		err = screenConversationRequire(query, conversationID)
	}
	if err != nil {
		screenWriteError(r, err)
		return
	}
	query = query.Where("conversation_id = ?", conversationID)
	if before := r.GetQuery("before").Uint(); before > 0 {
		query = query.Where("id < ?", before)
	}
	var rows []store.IOCScreenMessage
	if err = query.Order("id DESC").Limit(101).Find(&rows).Error; err != nil {
		screenWriteError(r, err)
		return
	}
	hasMore := len(rows) > 100
	if hasMore {
		rows = rows[:100]
	}
	items := []json.RawMessage{}
	var before uint
	for i := len(rows) - 1; i >= 0; i-- {
		if before == 0 {
			before = rows[i].ID
		}
		items = append(items, json.RawMessage(rows[i].Payload))
	}
	screenWriteData(r, map[string]interface{}{"items": items, "before": before, "hasMore": hasMore})
}

func (s *Server) handleScreenAssistantMessage(r *ghttp.Request) {
	row, err := screenLoad(r, "screen:edit")
	if err != nil {
		screenWriteError(r, err)
		return
	}
	var message screenConversationMessage
	if err = screenReadJSON(r, &message); err != nil {
		screenWriteError(r, err)
		return
	}
	valid := screenAuditRequestID.MatchString(message.ID) && (message.Role == "user" || message.Role == "assistant") && len(message.Content) <= 65536 && len(message.Context) <= 512<<10 && len(message.Stages) <= 16 && message.ChangeCount >= 0 && message.ChangeCount <= 201 && (message.TaskID == "" || screenAuditRequestID.MatchString(message.TaskID))
	switch message.Status {
	case "", "done", "generating", "ready", "applied", "undone", "failed", "interrupted", "waiting", "canceled", "discarded":
	default:
		valid = false
	}
	for _, stage := range message.Stages {
		switch stage {
		case "context", "discovering", "planning", "intention", "metrics", "generating", "reviewing", "validating", "completed", "applying", "applied", "undone":
		default:
			valid = false
		}
	}
	if !valid {
		screenWriteError(r, screenFail(400, "Invalid conversation message / 对话记录无效"))
		return
	}
	conversationID, err := screenConversationStorageID(message.ConversationID)
	if err != nil {
		screenWriteError(r, err)
		return
	}
	if conversationID == "" {
		message.ConversationID = "legacy"
	}
	encoded, err := json.Marshal(message)
	if err != nil || len(encoded) > 640<<10 {
		screenWriteError(r, screenFail(413, "Conversation message exceeds limit / 对话内容超过限制"))
		return
	}
	stored := store.IOCScreenMessage{TenantID: row.TenantID, ProjectID: row.ProjectID, ScreenID: row.ID, UserID: requestAuthContext(r).UserID, ConversationID: conversationID, MessageID: message.ID, Payload: string(encoded)}
	err = store.DB.Transaction(func(tx *gorm.DB) error {
		scope := tx.Where("tenant_id = ? AND project_id = ? AND screen_id = ? AND user_id = ?", row.TenantID, row.ProjectID, row.ID, requestAuthContext(r).UserID).Session(&gorm.Session{})
		if err := screenConversationRequire(scope, conversationID); err != nil {
			return err
		}
		// A retry may update its own message, but cannot move an existing message
		// to another conversation or overwrite that conversation's payload.
		result := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "tenant_id"}, {Name: "project_id"}, {Name: "screen_id"}, {Name: "user_id"}, {Name: "message_id"}}, DoUpdates: clause.Assignments(map[string]interface{}{"payload": stored.Payload, "updated_at": time.Now()}), Where: clause.Where{Exprs: []clause.Expression{clause.Eq{Column: clause.Column{Table: clause.CurrentTable, Name: "conversation_id"}, Value: conversationID}}}}).Create(&stored)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return screenFail(409, "Message belongs to another conversation / 消息属于其他对话")
		}
		id := conversationID
		if id == "" {
			id = "legacy"
			legacy := screenConversationOwner(r, row, id)
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&legacy).Error; err != nil {
				return err
			}
		}
		if message.Role == "user" && strings.TrimSpace(message.Content) != "" {
			if err := scope.Model(&store.IOCScreenConversation{}).Where("conversation_id = ? AND title = ?", id, "").Update("title", screenAssistantText(strings.Join(strings.Fields(message.Content), " "), 80)).Error; err != nil {
				return err
			}
		}
		return scope.Model(&store.IOCScreenConversation{}).Where("conversation_id = ?", id).Update("updated_at", time.Now()).Error
	})
	if err != nil {
		screenWriteError(r, err)
		return
	}
	screenWriteData(r, map[string]interface{}{"saved": true})
}
