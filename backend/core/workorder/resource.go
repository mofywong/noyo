package workorder

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
	"noyo/core/store"
)

var (
	ErrResourceNotFound     = errors.New("resource not found")
	ErrResourceTypeMismatch = errors.New("resource type mismatch")
)

type ResourceReference struct {
	Type  string `json:"type"`
	ID    string `json:"id"`
	Field string `json:"field,omitempty"`
}

type Resource struct {
	Type      string
	ID        string
	TenantID  uint
	ProjectID uint
}

type ResourceAuditEvent struct {
	Type      string
	ID        string
	Field     string
	TenantID  uint
	ProjectID uint
	Code      string
	Message   string
}

type ResourceResolver interface {
	Resolve(context.Context, Scope, ResourceReference) (Resource, error)
}

type ResourceAuditWriter interface {
	WriteResourceAudit(context.Context, ResourceAuditEvent) error
}

type ResourceValidator struct {
	resolver ResourceResolver
	audit    ResourceAuditWriter
}

func NewResourceValidator(resolver ResourceResolver, audit ResourceAuditWriter) *ResourceValidator {
	return &ResourceValidator{resolver: resolver, audit: audit}
}

func (v *ResourceValidator) Validate(ctx context.Context, scope Scope, refs []ResourceReference) error {
	if scope.TenantID == 0 || scope.ProjectID == 0 {
		return v.fail(ctx, scope, ResourceReference{}, CodeValidationFailed, "tenant_id and project_id are required")
	}
	if v == nil || v.resolver == nil {
		return NewResourceError(CodeValidationFailed, "resource resolver is unavailable")
	}
	for _, ref := range refs {
		ref.Type = strings.ToLower(strings.TrimSpace(ref.Type))
		ref.ID = strings.TrimSpace(ref.ID)
		if !supportedResourceType(ref.Type) || ref.ID == "" {
			return v.fail(ctx, scope, ref, CodeValidationFailed, "resource type and id are required")
		}
		resource, err := v.resolver.Resolve(ctx, scope, ref)
		if err != nil {
			code := CodeValidationFailed
			if errors.Is(err, ErrResourceTypeMismatch) {
				code = CodeValidationFailed
			}
			return v.fail(ctx, scope, ref, code, err.Error())
		}
		if resource.Type != "" && resource.Type != ref.Type {
			return v.fail(ctx, scope, ref, CodeValidationFailed, ErrResourceTypeMismatch.Error())
		}
		if resource.TenantID != scope.TenantID || resource.ProjectID != scope.ProjectID {
			return v.fail(ctx, scope, ref, CodePermissionDenied, "resource belongs to another tenant or project")
		}
	}
	return nil
}

func (v *ResourceValidator) fail(ctx context.Context, scope Scope, ref ResourceReference, code, message string) error {
	if v != nil && v.audit != nil && ref.Type != "" && ref.ID != "" {
		_ = v.audit.WriteResourceAudit(ctx, ResourceAuditEvent{Type: ref.Type, ID: ref.ID, Field: ref.Field, TenantID: scope.TenantID, ProjectID: scope.ProjectID, Code: code, Message: message})
	}
	return NewResourceError(code, message)
}

type ResourceError struct {
	Code    string
	Message string
}

func (e *ResourceError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}
func NewResourceError(code, message string) *ResourceError {
	return &ResourceError{Code: code, Message: message}
}
func IsResourceErrorCode(err error, code string) bool {
	var target *ResourceError
	return errors.As(err, &target) && target.Code == code
}

// GormResourceResolver is the production resolver used by the work-order
// service. Device and user IDs are public codes/usernames; no database ID is
// placed in a form DTO or audit payload.
type GormResourceResolver struct{ DB *gorm.DB }

func NewGormResourceResolver(db *gorm.DB) *GormResourceResolver { return &GormResourceResolver{DB: db} }

func (r *GormResourceResolver) Resolve(_ context.Context, scope Scope, ref ResourceReference) (Resource, error) {
	if r == nil || r.DB == nil {
		return Resource{}, ErrResourceNotFound
	}
	if scope.TenantID == 0 || scope.ProjectID == 0 {
		return Resource{}, ErrResourceNotFound
	}
	switch strings.ToLower(strings.TrimSpace(ref.Type)) {
	case "device":
		var device store.Device
		if err := r.DB.Where("tenant_id = ? AND project_id = ? AND code = ?", scope.TenantID, scope.ProjectID, ref.ID).First(&device).Error; err != nil {
			return Resource{}, ErrResourceNotFound
		}
		return Resource{Type: "device", ID: device.Code, TenantID: device.TenantID, ProjectID: device.ProjectID}, nil
	case "user":
		var user store.User
		if err := r.DB.Where("tenant_id = ? AND (username = ? OR CAST(id AS TEXT) = ?)", scope.TenantID, ref.ID, ref.ID).First(&user).Error; err != nil {
			return Resource{}, ErrResourceNotFound
		}
		var count int64
		if err := r.DB.Model(&store.UserRoleBinding{}).Where("user_id = ? AND tenant_id = ? AND project_id = ?", user.ID, scope.TenantID, scope.ProjectID).Count(&count).Error; err != nil || count == 0 {
			return Resource{}, ErrResourceNotFound
		}
		return Resource{Type: "user", ID: user.Username, TenantID: user.TenantID, ProjectID: scope.ProjectID}, nil
	case "space", "alarm":
		// Space and alarm persistence is provided by protocol-specific modules;
		// use a scoped table lookup when those models are installed.
		var row struct {
			ID        string
			TenantID  uint `gorm:"column:tenant_id"`
			ProjectID uint `gorm:"column:project_id"`
		}
		table := "spaces"
		if ref.Type == "alarm" {
			table = "alarms"
		}
		if err := r.DB.Table(table).Select("id, tenant_id, project_id").Where("tenant_id = ? AND project_id = ? AND id = ?", scope.TenantID, scope.ProjectID, ref.ID).First(&row).Error; err != nil {
			return Resource{}, ErrResourceNotFound
		}
		return Resource{Type: ref.Type, ID: ref.ID, TenantID: row.TenantID, ProjectID: row.ProjectID}, nil
	default:
		return Resource{}, ErrResourceTypeMismatch
	}
}

type StoreAuditWriter struct {
	DB     *gorm.DB
	UserID uint
}

func (w StoreAuditWriter) WriteResourceAudit(_ context.Context, event ResourceAuditEvent) error {
	if w.DB == nil {
		return nil
	}
	return w.DB.Create(&store.AuditLog{TenantID: event.TenantID, UserID: w.UserID, Module: "work_order", Action: "resource_validate", Resource: event.Type + ":" + event.ID, Detail: event.Code + ": " + event.Message}).Error
}
