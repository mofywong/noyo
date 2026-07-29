package workorder

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"noyo/core/store"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// GormIntegrationStore makes inbound delivery receipts and their revision
// cursor durable. It intentionally keeps transport concerns outside this
// package: callers can use the persisted work-order outbox for delivery.
type GormIntegrationStore struct {
	db *gorm.DB
}

func NewGormIntegrationStore(db *gorm.DB) *GormIntegrationStore {
	return &GormIntegrationStore{db: db}
}

func (s *GormIntegrationStore) SaveConnector(connector Connector) (Connector, error) {
	connector.ID = strings.TrimSpace(connector.ID)
	connector.Name = strings.TrimSpace(connector.Name)
	connector.KeyID = strings.TrimSpace(connector.KeyID)
	if connector.ID == "" {
		connector.ID = uuid.NewString()
	}
	if connector.TenantID == 0 || connector.ProjectID == 0 || connector.Name == "" || connector.KeyID == "" {
		return Connector{}, NewError(CodeValidationFailed, "connector name, key id, and tenant/project scope are required")
	}
	if connector.SecretHash == "" {
		return Connector{}, NewError(CodeValidationFailed, "connector secret is required")
	}
	record := store.WorkOrderIntegrationConnector{PublicID: connector.ID, TenantID: connector.TenantID, ProjectID: connector.ProjectID,
		Name: connector.Name, KeyID: connector.KeyID, SecretHash: connector.SecretHash, Enabled: connector.Enabled}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var existing store.WorkOrderIntegrationConnector
		err := tx.Where("public_id = ?", connector.ID).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return tx.Create(&record).Error
		}
		if err != nil {
			return fmt.Errorf("load integration connector: %w", err)
		}
		if existing.TenantID != connector.TenantID || existing.ProjectID != connector.ProjectID {
			return NewError(CodePermissionDenied, "connector belongs to another scope")
		}
		record.ID, record.CreatedAt = existing.ID, existing.CreatedAt
		return tx.Save(&record).Error
	})
	if err != nil {
		return Connector{}, fmt.Errorf("save integration connector: %w", err)
	}
	return connector, nil
}

func (s *GormIntegrationStore) ListConnectors(scope Scope) ([]Connector, error) {
	if err := validateAlarmScope(scope); err != nil {
		return nil, err
	}
	var records []store.WorkOrderIntegrationConnector
	if err := s.db.Where("tenant_id = ? AND project_id = ?", scope.TenantID, scope.ProjectID).Order("created_at DESC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list integration connectors: %w", err)
	}
	connectors := make([]Connector, 0, len(records))
	for _, record := range records {
		connectors = append(connectors, Connector{ID: record.PublicID, Name: record.Name, KeyID: record.KeyID,
			TenantID: record.TenantID, ProjectID: record.ProjectID, Enabled: record.Enabled})
	}
	return connectors, nil
}

func (s *GormIntegrationStore) PutBinding(binding Binding) {
	_, _ = s.SaveBinding(binding)
}

func (s *GormIntegrationStore) SaveBinding(binding Binding) (Binding, error) {
	binding.ID = strings.TrimSpace(binding.ID)
	binding.ConnectorID = strings.TrimSpace(binding.ConnectorID)
	binding.RemoteProject = strings.TrimSpace(binding.RemoteProject)
	binding.Origin = strings.TrimSpace(binding.Origin)
	binding.Direction = strings.TrimSpace(binding.Direction)
	if binding.ID == "" || binding.TenantID == 0 || binding.ProjectID == 0 {
		return Binding{}, NewError(CodeValidationFailed, "binding id and tenant/project scope are required")
	}
	if binding.Direction == "" {
		binding.Direction = "bidirectional"
	}
	if binding.Direction != "inbound" && binding.Direction != "outbound" && binding.Direction != "bidirectional" {
		return Binding{}, NewError(CodeValidationFailed, "binding direction must be inbound, outbound, or bidirectional")
	}
	if binding.ConnectorID == "" {
		return Binding{}, NewError(CodeValidationFailed, "connector id is required")
	}
	record := store.WorkOrderIntegrationBinding{
		PublicID: binding.ID, TenantID: binding.TenantID, ProjectID: binding.ProjectID,
		ConnectorPublicID: binding.ConnectorID, RemoteProject: binding.RemoteProject,
		Origin: binding.Origin, Direction: binding.Direction, LastRevision: binding.LastRevision,
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var connector store.WorkOrderIntegrationConnector
		if err := tx.Where("public_id = ? AND tenant_id = ? AND project_id = ? AND enabled = ?", binding.ConnectorID, binding.TenantID, binding.ProjectID, true).First(&connector).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return NewError(CodeWorkOrderNotFound, "enabled integration connector not found")
			}
			return fmt.Errorf("load integration connector: %w", err)
		}
		var existing store.WorkOrderIntegrationBinding
		err := tx.Where("public_id = ?", binding.ID).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return tx.Create(&record).Error
		}
		if err != nil {
			return fmt.Errorf("load integration binding: %w", err)
		}
		if existing.TenantID != binding.TenantID || existing.ProjectID != binding.ProjectID {
			return NewError(CodePermissionDenied, "binding belongs to another scope")
		}
		record.ID = existing.ID
		record.CreatedAt = existing.CreatedAt
		return tx.Save(&record).Error
	})
	if err != nil {
		return Binding{}, fmt.Errorf("save integration binding: %w", err)
	}
	return binding, nil
}

func (s *GormIntegrationStore) ListBindings(scope Scope) ([]Binding, error) {
	if err := validateAlarmScope(scope); err != nil {
		return nil, err
	}
	var records []store.WorkOrderIntegrationBinding
	if err := s.db.Where("tenant_id = ? AND project_id = ?", scope.TenantID, scope.ProjectID).Order("created_at DESC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list integration bindings: %w", err)
	}
	bindings := make([]Binding, 0, len(records))
	for _, record := range records {
		bindings = append(bindings, Binding{ID: record.PublicID, ConnectorID: record.ConnectorPublicID, RemoteProject: record.RemoteProject,
			TenantID: record.TenantID, ProjectID: record.ProjectID, Origin: record.Origin, Direction: record.Direction, LastRevision: record.LastRevision})
	}
	return bindings, nil
}

func (s *GormIntegrationStore) Ingest(binding Binding, messageID string, revision int64, body []byte, apply func() error) (InboxMessage, error) {
	binding.ID = strings.TrimSpace(binding.ID)
	messageID = strings.TrimSpace(messageID)
	if binding.ID == "" || messageID == "" || binding.TenantID == 0 || binding.ProjectID == 0 {
		return InboxMessage{}, NewError(CodeValidationFailed, "binding, message id, and tenant/project scope are required")
	}
	digestBytes := sha256.Sum256(body)
	digest := hex.EncodeToString(digestBytes[:])
	var result InboxMessage
	var reserved store.WorkOrderIntegrationInboxMessage
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var persistedBinding store.WorkOrderIntegrationBinding
		if err := tx.Where("public_id = ? AND tenant_id = ? AND project_id = ?", binding.ID, binding.TenantID, binding.ProjectID).First(&persistedBinding).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return NewError(CodeWorkOrderNotFound, "integration binding not found")
			}
			return fmt.Errorf("load integration binding: %w", err)
		}
		var existing store.WorkOrderIntegrationInboxMessage
		err := tx.Where("tenant_id = ? AND project_id = ? AND binding_public_id = ? AND message_id = ?", binding.TenantID, binding.ProjectID, binding.ID, messageID).First(&existing).Error
		if err == nil {
			if existing.Digest != digest {
				return NewError(CodeIdempotencyConflict, "message digest changed")
			}
			result = inboxMessageFromStore(existing)
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("load integration inbox message: %w", err)
		}
		now := time.Now().UTC()
		record := store.WorkOrderIntegrationInboxMessage{
			TenantID: binding.TenantID, ProjectID: binding.ProjectID, BindingPublicID: binding.ID,
			MessageID: messageID, Digest: digest, Revision: revision, ReceivedAt: now,
		}
		if revision < persistedBinding.LastRevision {
			record.Status = InboxDuplicate
			if err := tx.Create(&record).Error; err != nil {
				return fmt.Errorf("persist stale integration inbox message: %w", err)
			}
			result = inboxMessageFromStore(record)
			return nil
		}
		record.Status = InboxProcessing
		if err := tx.Create(&record).Error; err != nil {
			return fmt.Errorf("reserve integration inbox message: %w", err)
		}
		reserved = record
		result = inboxMessageFromStore(record)
		return nil
	})
	if err != nil {
		return result, err
	}
	if result.Status != InboxProcessing {
		return result, nil
	}
	applyErr := error(nil)
	if apply != nil {
		applyErr = apply()
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		status := InboxApplied
		errorMessage := ""
		if applyErr != nil {
			status = InboxRejected
			errorMessage = applyErr.Error()
		}
		query := tx.Model(&store.WorkOrderIntegrationInboxMessage{}).Where("id = ? AND status = ?", reserved.ID, InboxProcessing).
			Updates(map[string]any{"status": status, "error": errorMessage})
		if query.Error != nil {
			return fmt.Errorf("complete integration inbox message: %w", query.Error)
		}
		if query.RowsAffected != 1 {
			return NewError(CodeIdempotencyConflict, "integration inbox message was changed concurrently")
		}
		if applyErr == nil {
			if err := tx.Model(&store.WorkOrderIntegrationBinding{}).
				Where("public_id = ? AND tenant_id = ? AND project_id = ? AND last_revision <= ?", binding.ID, binding.TenantID, binding.ProjectID, revision).
				Update("last_revision", revision).Error; err != nil {
				return fmt.Errorf("advance integration binding revision: %w", err)
			}
		}
		reserved.Status = status
		reserved.Error = errorMessage
		result = inboxMessageFromStore(reserved)
		return nil
	})
	if err != nil {
		return result, err
	}
	if applyErr != nil {
		return result, applyErr
	}
	return result, nil
}

func (s *GormIntegrationStore) SaveMapping(scope Scope, bindingID string, mapping Mapping) (Mapping, error) {
	if err := validateAlarmScope(scope); err != nil {
		return Mapping{}, err
	}
	bindingID = strings.TrimSpace(bindingID)
	if bindingID == "" {
		return Mapping{}, NewError(CodeValidationFailed, "binding id is required")
	}
	if err := ValidateMapping(mapping); err != nil {
		return Mapping{}, NewError(CodeValidationFailed, err.Error())
	}
	fields, err := json.Marshal(mapping.Fields)
	if err != nil {
		return Mapping{}, NewError(CodeValidationFailed, "mapping fields must be JSON serializable")
	}
	statuses, err := json.Marshal(mapping.Status)
	if err != nil {
		return Mapping{}, NewError(CodeValidationFailed, "mapping status must be JSON serializable")
	}
	priorities, err := json.Marshal(mapping.Priority)
	if err != nil {
		return Mapping{}, NewError(CodeValidationFailed, "mapping priority must be JSON serializable")
	}
	mapping.ID = strings.TrimSpace(mapping.ID)
	mapping.Name = strings.TrimSpace(mapping.Name)
	if mapping.ID == "" || mapping.Name == "" {
		return Mapping{}, NewError(CodeValidationFailed, "mapping id and name are required")
	}
	record := store.WorkOrderIntegrationMapping{PublicID: mapping.ID, TenantID: scope.TenantID, ProjectID: scope.ProjectID, BindingPublicID: bindingID,
		Name: mapping.Name, Revision: mapping.Revision, FieldsJSON: string(fields), StatusJSON: string(statuses), PriorityJSON: string(priorities)}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var binding store.WorkOrderIntegrationBinding
		if err := tx.Where("public_id = ? AND tenant_id = ? AND project_id = ?", bindingID, scope.TenantID, scope.ProjectID).First(&binding).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return NewError(CodeWorkOrderNotFound, "integration binding not found")
			}
			return err
		}
		var existing store.WorkOrderIntegrationMapping
		err := tx.Where("public_id = ?", mapping.ID).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return tx.Create(&record).Error
		}
		if err != nil {
			return err
		}
		if existing.TenantID != scope.TenantID || existing.ProjectID != scope.ProjectID {
			return NewError(CodePermissionDenied, "mapping belongs to another scope")
		}
		record.ID, record.CreatedAt = existing.ID, existing.CreatedAt
		return tx.Save(&record).Error
	})
	if err != nil {
		return Mapping{}, fmt.Errorf("save integration mapping: %w", err)
	}
	return mapping, nil
}

func (s *GormIntegrationStore) ListMappings(scope Scope, bindingID string) ([]Mapping, error) {
	if err := validateAlarmScope(scope); err != nil {
		return nil, err
	}
	var records []store.WorkOrderIntegrationMapping
	if err := s.db.Where("tenant_id = ? AND project_id = ? AND binding_public_id = ?", scope.TenantID, scope.ProjectID, strings.TrimSpace(bindingID)).Order("revision DESC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list integration mappings: %w", err)
	}
	mappings := make([]Mapping, 0, len(records))
	for _, record := range records {
		mapping, err := mappingFromStore(record)
		if err != nil {
			return nil, err
		}
		mappings = append(mappings, mapping)
	}
	return mappings, nil
}

func inboxMessageFromStore(record store.WorkOrderIntegrationInboxMessage) InboxMessage {
	return InboxMessage{BindingID: record.BindingPublicID, MessageID: record.MessageID, Digest: record.Digest, Revision: record.Revision,
		Status: record.Status, Error: record.Error, ReceivedAt: record.ReceivedAt}
}

func mappingFromStore(record store.WorkOrderIntegrationMapping) (Mapping, error) {
	mapping := Mapping{ID: record.PublicID, Name: record.Name, Revision: record.Revision, Fields: map[string]string{}, Status: map[string]string{}, Priority: map[string]string{}}
	if err := json.Unmarshal([]byte(record.FieldsJSON), &mapping.Fields); err != nil {
		return Mapping{}, fmt.Errorf("decode integration mapping fields: %w", err)
	}
	if err := json.Unmarshal([]byte(record.StatusJSON), &mapping.Status); err != nil {
		return Mapping{}, fmt.Errorf("decode integration mapping status: %w", err)
	}
	if err := json.Unmarshal([]byte(record.PriorityJSON), &mapping.Priority); err != nil {
		return Mapping{}, fmt.Errorf("decode integration mapping priority: %w", err)
	}
	return mapping, nil
}
