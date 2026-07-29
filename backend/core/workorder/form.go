package workorder

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	FormVersionDraft     = "draft"
	FormVersionPublished = "published"
)

var supportedSchemaTypes = map[string]bool{
	"string": true, "number": true, "integer": true, "boolean": true,
	"enum": true, "date-time": true, "array": true,
}

// FormDefinition is the public, database-independent form document. Schema is
// deliberately a map so the HTTP layer can decode JSON without exposing a
// GORM model or an internal numeric identifier.
type FormDefinition struct {
	Schema     map[string]any `json:"schema"`
	UISchema   map[string]any `json:"ui_schema,omitempty"`
	Defaults   map[string]any `json:"defaults,omitempty"`
	Required   []string       `json:"required,omitempty"`
	FieldOrder []string       `json:"field_order,omitempty"`
}

type ResourceField struct {
	Field string `json:"field"`
	Type  string `json:"type"`
}

type CompiledForm struct {
	Definition     FormDefinition
	Checksum       string
	SnapshotJSON   []byte
	Fields         []string
	ResourceFields []ResourceField
}

// FormVersion is a safe DTO. It intentionally has no gorm.Model or database
// primary key; ID is an opaque public identifier used by the form port.
type FormVersion struct {
	ID           string         `json:"id"`
	TemplateCode string         `json:"template_code"`
	Version      int            `json:"version"`
	Status       string         `json:"status"`
	Checksum     string         `json:"checksum"`
	Schema       map[string]any `json:"schema"`
	UISchema     map[string]any `json:"ui_schema,omitempty"`
	Defaults     map[string]any `json:"defaults,omitempty"`
	Required     []string       `json:"required,omitempty"`
	FieldOrder   []string       `json:"field_order,omitempty"`
	PublishedAt  *time.Time     `json:"published_at,omitempty"`
}

func ValidateFormDefinition(definition FormDefinition) error {
	_, err := CompileFormDefinition(definition)
	return err
}

func CompileFormDefinition(definition FormDefinition) (CompiledForm, error) {
	if len(definition.Schema) == 0 {
		return CompiledForm{}, NewError(CodeValidationFailed, "schema is required")
	}
	rootType, _ := definition.Schema["type"].(string)
	if rootType != "object" {
		return CompiledForm{}, NewError(CodeValidationFailed, "schema root type must be object")
	}
	for key := range definition.Schema {
		switch key {
		case "type", "properties", "required", "title", "description":
		default:
			return CompiledForm{}, NewError(CodeValidationFailed, fmt.Sprintf("schema uses unsupported keyword %q", key))
		}
	}
	properties, ok := definition.Schema["properties"].(map[string]any)
	if !ok {
		return CompiledForm{}, NewError(CodeValidationFailed, "schema properties must be an object")
	}

	fields := make([]string, 0, len(properties))
	resourceFields := make([]ResourceField, 0)
	for name, raw := range properties {
		if strings.TrimSpace(name) == "" {
			return CompiledForm{}, NewError(CodeValidationFailed, "schema field name cannot be empty")
		}
		field, ok := raw.(map[string]any)
		if !ok {
			return CompiledForm{}, NewError(CodeValidationFailed, fmt.Sprintf("schema field %q must be an object", name))
		}
		fieldType, _ := field["type"].(string)
		if !supportedSchemaTypes[fieldType] {
			return CompiledForm{}, NewError(CodeValidationFailed, fmt.Sprintf("schema field %q has unsupported type %q", name, fieldType))
		}
		for key := range field {
			switch key {
			case "type", "format", "enum", "items", "default", "title", "description", "x-noyo-resource", "x-noyo-attachment", "x-noyo-images":
			default:
				return CompiledForm{}, NewError(CodeValidationFailed, fmt.Sprintf("schema field %q uses unsupported keyword %q", name, key))
			}
		}
		if format, present := field["format"]; present && format != "date-time" {
			return CompiledForm{}, NewError(CodeValidationFailed, fmt.Sprintf("schema field %q has unsupported format", name))
		}
		if fieldType == "string" && field["format"] == "date-time" {
			fieldType = "date-time"
		}
		if fieldType == "string" && validEnum(field["enum"]) {
			fieldType = "enum"
		}
		if fieldType == "enum" {
			if !validEnum(field["enum"]) {
				return CompiledForm{}, NewError(CodeValidationFailed, fmt.Sprintf("schema enum field %q requires a non-empty enum", name))
			}
		}
		images, _ := field["x-noyo-images"].(bool)
		if fieldType == "array" && ((!images && !validArrayEnum(field["items"])) || (images && !validStringArrayItems(field["items"]))) {
			return CompiledForm{}, NewError(CodeValidationFailed, fmt.Sprintf("schema array field %q requires valid string items", name))
		}
		if resourceType, present := field["x-noyo-resource"]; present {
			resource, ok := resourceType.(string)
			if !ok || !supportedResourceType(resource) {
				return CompiledForm{}, NewError(CodeValidationFailed, fmt.Sprintf("schema resource field %q has unsupported resource type", name))
			}
			resourceFields = append(resourceFields, ResourceField{Field: name, Type: resource})
		}
		if attachment, present := field["x-noyo-attachment"]; present {
			if enabled, ok := attachment.(bool); !ok || !enabled || fieldType != "string" {
				return CompiledForm{}, NewError(CodeValidationFailed, fmt.Sprintf("schema attachment field %q must be a string attachment", name))
			}
		}
		if imageUpload, present := field["x-noyo-images"]; present {
			if enabled, ok := imageUpload.(bool); !ok || !enabled || fieldType != "array" || !validStringArrayItems(field["items"]) {
				return CompiledForm{}, NewError(CodeValidationFailed, fmt.Sprintf("schema image field %q must be an array of strings", name))
			}
		}
		if value, present := field["default"]; present {
			if err := validateValue(fieldType, validationOptions(field), value); err != nil {
				return CompiledForm{}, NewError(CodeValidationFailed, fmt.Sprintf("schema default for %q: %v", name, err))
			}
		}
		fields = append(fields, name)
	}
	sort.Strings(fields)

	required := append([]string(nil), definition.Required...)
	if len(required) == 0 {
		required = stringSlice(definition.Schema["required"])
	}
	if err := validateFieldList(required, properties, "required"); err != nil {
		return CompiledForm{}, err
	}
	fieldOrder := append([]string(nil), definition.FieldOrder...)
	if len(fieldOrder) == 0 {
		fieldOrder = append([]string(nil), fields...)
	}
	if err := validateFieldList(fieldOrder, properties, "field_order"); err != nil {
		return CompiledForm{}, err
	}
	if len(uniqueStrings(fieldOrder)) != len(fieldOrder) {
		return CompiledForm{}, NewError(CodeValidationFailed, "field_order contains duplicate fields")
	}

	defaults := cloneMap(definition.Defaults)
	if defaults == nil {
		defaults = map[string]any{}
	}
	for name, raw := range properties {
		field := raw.(map[string]any)
		if _, exists := defaults[name]; !exists {
			if value, hasSchemaDefault := field["default"]; hasSchemaDefault {
				defaults[name] = deepClone(value)
			}
		}
	}
	for name, value := range defaults {
		raw, exists := properties[name]
		if !exists {
			return CompiledForm{}, NewError(CodeValidationFailed, fmt.Sprintf("default references unknown field %q", name))
		}
		field := raw.(map[string]any)
		if err := validateValue(schemaFieldType(field), validationOptions(field), value); err != nil {
			return CompiledForm{}, NewError(CodeValidationFailed, fmt.Sprintf("default for %q: %v", name, err))
		}
	}
	if err := validateUISchema(definition.UISchema, properties); err != nil {
		return CompiledForm{}, err
	}

	canonical := struct {
		Schema     map[string]any `json:"schema"`
		UISchema   map[string]any `json:"ui_schema,omitempty"`
		Defaults   map[string]any `json:"defaults,omitempty"`
		Required   []string       `json:"required,omitempty"`
		FieldOrder []string       `json:"field_order,omitempty"`
	}{definition.Schema, definition.UISchema, defaults, required, fieldOrder}
	snapshot, err := json.Marshal(canonical)
	if err != nil {
		return CompiledForm{}, NewError(CodeValidationFailed, "encode canonical form snapshot")
	}
	digest := sha256.Sum256(snapshot)
	return CompiledForm{
		Definition: FormDefinition{
			Schema: cloneMap(definition.Schema), UISchema: cloneMap(definition.UISchema),
			Defaults: cloneMap(defaults), Required: append([]string(nil), required...), FieldOrder: append([]string(nil), fieldOrder...),
		},
		Checksum: hex.EncodeToString(digest[:]), SnapshotJSON: append([]byte(nil), snapshot...),
		Fields: append([]string(nil), fieldOrder...), ResourceFields: append([]ResourceField(nil), resourceFields...),
	}, nil
}

func CanonicalFormChecksum(definition FormDefinition) (string, error) {
	compiled, err := CompileFormDefinition(definition)
	if err != nil {
		return "", err
	}
	return compiled.Checksum, nil
}

func ValidateFormData(compiled CompiledForm, values map[string]any) error {
	properties := compiled.Definition.Schema["properties"].(map[string]any)
	for name, value := range values {
		raw, exists := properties[name]
		if !exists {
			return NewError(CodeValidationFailed, fmt.Sprintf("form data references unknown field %q", name))
		}
		field := raw.(map[string]any)
		if err := validateValue(schemaFieldType(field), validationOptions(field), value); err != nil {
			return NewError(CodeValidationFailed, fmt.Sprintf("form data for %q: %v", name, err))
		}
	}
	for _, name := range compiled.Definition.Required {
		if value, ok := values[name]; !ok || !formValuePresent(value) {
			return NewError(CodeValidationFailed, fmt.Sprintf("required form field %q is missing", name))
		}
	}
	return nil
}

func ExtractResourceReferences(compiled CompiledForm, values map[string]any) []ResourceReference {
	refs := make([]ResourceReference, 0)
	for _, field := range compiled.ResourceFields {
		value, ok := values[field.Field]
		if !ok {
			continue
		}
		for _, id := range stringValues(value) {
			if strings.TrimSpace(id) != "" {
				refs = append(refs, ResourceReference{Type: field.Type, ID: id, Field: field.Field})
			}
		}
	}
	return refs
}

type FormVersionService struct {
	mu       sync.RWMutex
	nextID   uint64
	versions map[string]FormVersion
	byScope  map[string][]string
}

func NewFormVersionService() *FormVersionService {
	return &FormVersionService{versions: make(map[string]FormVersion), byScope: make(map[string][]string)}
}

func (s *FormVersionService) CreateDraft(scope Scope, templateCode string, definition FormDefinition) (FormVersion, error) {
	if err := validateFormScope(scope); err != nil {
		return FormVersion{}, err
	}
	if strings.TrimSpace(templateCode) == "" {
		return FormVersion{}, NewError(CodeValidationFailed, "template code is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	version := FormVersion{ID: fmt.Sprintf("form-%d", s.nextID), TemplateCode: templateCode, Version: len(s.byScope[scopeKey(scope, templateCode)]) + 1, Status: FormVersionDraft, Schema: cloneMap(definition.Schema), UISchema: cloneMap(definition.UISchema), Defaults: cloneMap(definition.Defaults), Required: append([]string(nil), definition.Required...), FieldOrder: append([]string(nil), definition.FieldOrder...)}
	s.versions[version.ID] = version
	key := scopeKey(scope, templateCode)
	s.byScope[key] = append(s.byScope[key], version.ID)
	return cloneFormVersion(version), nil
}

func (s *FormVersionService) UpdateDraft(scope Scope, id string, definition FormDefinition) (FormVersion, error) {
	if err := validateFormScope(scope); err != nil {
		return FormVersion{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	version, ok := s.versions[id]
	if !ok || !scopeOwnsVersion(scope, version, s) {
		return FormVersion{}, NewError(CodePermissionDenied, "form version is not available in this scope")
	}
	if version.Status != FormVersionDraft {
		return FormVersion{}, NewError(CodeValidationFailed, "published form version is immutable")
	}
	version.Schema, version.UISchema, version.Defaults = cloneMap(definition.Schema), cloneMap(definition.UISchema), cloneMap(definition.Defaults)
	version.Required, version.FieldOrder = append([]string(nil), definition.Required...), append([]string(nil), definition.FieldOrder...)
	s.versions[id] = version
	return cloneFormVersion(version), nil
}

func (s *FormVersionService) Publish(scope Scope, id string) (FormVersion, error) {
	if err := validateFormScope(scope); err != nil {
		return FormVersion{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	version, ok := s.versions[id]
	if !ok || !scopeOwnsVersion(scope, version, s) {
		return FormVersion{}, NewError(CodePermissionDenied, "form version is not available in this scope")
	}
	if version.Status == FormVersionPublished {
		return cloneFormVersion(version), nil
	}
	definition := FormDefinition{Schema: version.Schema, UISchema: version.UISchema, Defaults: version.Defaults, Required: version.Required, FieldOrder: version.FieldOrder}
	compiled, err := CompileFormDefinition(definition)
	if err != nil {
		return FormVersion{}, err
	}
	now := time.Now().UTC()
	version.Status, version.Checksum, version.PublishedAt = FormVersionPublished, compiled.Checksum, &now
	version.Schema, version.UISchema, version.Defaults = cloneMap(compiled.Definition.Schema), cloneMap(compiled.Definition.UISchema), cloneMap(compiled.Definition.Defaults)
	version.Required, version.FieldOrder = append([]string(nil), compiled.Definition.Required...), append([]string(nil), compiled.Definition.FieldOrder...)
	s.versions[id] = version
	return cloneFormVersion(version), nil
}

func (s *FormVersionService) Get(scope Scope, id string) (FormVersion, error) {
	if err := validateFormScope(scope); err != nil {
		return FormVersion{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	version, ok := s.versions[id]
	if !ok || !scopeOwnsVersion(scope, version, s) {
		return FormVersion{}, NewError(CodePermissionDenied, "form version is not available in this scope")
	}
	return cloneFormVersion(version), nil
}

func validateFormScope(scope Scope) error {
	if scope.TenantID == 0 || scope.ProjectID == 0 {
		return NewError(CodeValidationFailed, "tenant_id and project_id are required")
	}
	return nil
}

func scopeKey(scope Scope, template string) string {
	return fmt.Sprintf("%d:%d:%s", scope.TenantID, scope.ProjectID, template)
}

func scopeOwnsVersion(scope Scope, version FormVersion, service *FormVersionService) bool {
	for _, id := range service.byScope[scopeKey(scope, version.TemplateCode)] {
		if id == version.ID {
			return true
		}
	}
	return false
}

func cloneFormVersion(version FormVersion) FormVersion {
	version.Schema, version.UISchema, version.Defaults = cloneMap(version.Schema), cloneMap(version.UISchema), cloneMap(version.Defaults)
	version.Required, version.FieldOrder = append([]string(nil), version.Required...), append([]string(nil), version.FieldOrder...)
	if version.PublishedAt != nil {
		t := *version.PublishedAt
		version.PublishedAt = &t
	}
	return version
}

func validateUISchema(ui map[string]any, properties map[string]any) error {
	if len(ui) == 0 {
		return nil
	}
	var fields []string
	if raw, ok := ui["order"]; ok {
		fields = stringSlice(raw)
	} else if raw, ok := ui["fields"]; ok {
		if list, ok := raw.([]any); ok {
			for _, item := range list {
				if object, ok := item.(map[string]any); ok {
					if name, ok := object["key"].(string); ok {
						fields = append(fields, name)
						continue
					}
					if name, ok := object["name"].(string); ok {
						fields = append(fields, name)
						continue
					}
				}
				if name, ok := item.(string); ok {
					fields = append(fields, name)
					continue
				}
				return NewError(CodeValidationFailed, "ui_schema fields must contain field names")
			}
		}
	}
	if len(fields) == 0 {
		if len(properties) == 0 {
			return nil
		}
		return NewError(CodeValidationFailed, "ui_schema must define order or fields")
	}
	return validateFieldList(fields, properties, "ui_schema")
}

func validateFieldList(fields []string, properties map[string]any, label string) error {
	for _, field := range fields {
		if strings.TrimSpace(field) == "" {
			return NewError(CodeValidationFailed, label+" contains an empty field")
		}
		if _, ok := properties[field]; !ok {
			return NewError(CodeValidationFailed, fmt.Sprintf("%s references unknown field %q", label, field))
		}
	}
	return nil
}

func validEnum(value any) bool {
	list, ok := value.([]any)
	return ok && len(list) > 0
}

func validArrayEnum(value any) bool {
	items, ok := value.(map[string]any)
	if !ok || items["type"] != "string" || !validEnum(items["enum"]) {
		return false
	}
	seen := make(map[string]bool)
	for _, raw := range items["enum"].([]any) {
		option, ok := raw.(string)
		if !ok || strings.TrimSpace(option) == "" || seen[option] {
			return false
		}
		seen[option] = true
	}
	return true
}

func validStringArrayItems(value any) bool {
	items, ok := value.(map[string]any)
	return ok && items["type"] == "string" && items["enum"] == nil
}

func formValuePresent(value any) bool {
	if value == nil {
		return false
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.String:
		return strings.TrimSpace(reflected.String()) != ""
	case reflect.Array, reflect.Slice:
		return reflected.Len() > 0
	default:
		return true
	}
}

func validationOptions(field map[string]any) any {
	if schemaFieldType(field) == "array" {
		return field["items"]
	}
	return field["enum"]
}

func schemaFieldType(field map[string]any) string {
	fieldType, _ := field["type"].(string)
	if fieldType == "string" && field["format"] == "date-time" {
		return "date-time"
	}
	if fieldType == "string" && validEnum(field["enum"]) {
		return "enum"
	}
	return fieldType
}

func validateValue(fieldType string, enum any, value any) error {
	if value == nil {
		return nil
	}
	switch fieldType {
	case "string", "date-time":
		text, ok := value.(string)
		if !ok {
			return errors.New("must be a string")
		}
		if fieldType == "date-time" {
			if _, err := time.Parse(time.RFC3339, text); err != nil {
				return errors.New("must be RFC3339 date-time")
			}
		}
	case "number":
		if !isNumber(value) {
			return errors.New("must be a number")
		}
	case "integer":
		if !isInteger(value) {
			return errors.New("must be an integer")
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return errors.New("must be a boolean")
		}
	case "enum":
		list, ok := enum.([]any)
		if !ok {
			return errors.New("enum values are required")
		}
		matched := false
		for _, option := range list {
			if reflect.DeepEqual(option, value) {
				matched = true
				break
			}
		}
		if !matched {
			return errors.New("must be one of the enum values")
		}
	case "array":
		items, ok := enum.(map[string]any)
		if !ok || (items["type"] != "string") {
			return errors.New("must define string items")
		}
		var configured []any
		if items["enum"] != nil {
			if !validArrayEnum(items) {
				return errors.New("must define valid string enum items")
			}
			configured = items["enum"].([]any)
		}
		var values []any
		switch raw := value.(type) {
		case []any:
			values = raw
		case []string:
			values = make([]any, len(raw))
			for index, item := range raw {
				values[index] = item
			}
		default:
			return errors.New("must be an array")
		}
		seen := make(map[string]bool, len(values))
		for _, value := range values {
			option, ok := value.(string)
			if !ok || strings.TrimSpace(option) == "" || seen[option] {
				return errors.New("must contain distinct non-empty strings")
			}
			if configured == nil {
				seen[option] = true
				continue
			}
			matched := false
			for _, candidate := range configured {
				if candidate == option {
					matched = true
					break
				}
			}
			if !matched {
				return errors.New("must contain only configured enum values")
			}
			seen[option] = true
		}
	}
	return nil
}

func isNumber(value any) bool {
	switch value.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64, json.Number:
		return true
	default:
		return false
	}
}
func isInteger(value any) bool {
	switch typed := value.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return true
	case float32:
		return float32(int64(typed)) == typed
	case float64:
		return float64(int64(typed)) == typed
	case json.Number:
		_, err := typed.Int64()
		return err == nil
	default:
		return false
	}
}
func supportedResourceType(value string) bool {
	return value == "device" || value == "user" || value == "space" || value == "alarm"
}
func stringSlice(value any) []string {
	list, ok := value.([]any)
	if !ok {
		if typed, ok := value.([]string); ok {
			return append([]string(nil), typed...)
		}
		return nil
	}
	result := make([]string, 0, len(list))
	for _, item := range list {
		if text, ok := item.(string); ok {
			result = append(result, text)
		}
	}
	return result
}
func stringValues(value any) []string {
	if text, ok := value.(string); ok {
		return []string{text}
	}
	if list, ok := value.([]any); ok {
		result := make([]string, 0, len(list))
		for _, item := range list {
			if text, ok := item.(string); ok {
				result = append(result, text)
			}
		}
		return result
	}
	return nil
}
func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}
func cloneMap(value map[string]any) map[string]any {
	if value == nil {
		return nil
	}
	cloned, ok := deepClone(value).(map[string]any)
	if !ok {
		return nil
	}
	return cloned
}
func deepClone(value any) any {
	encoded, err := json.Marshal(value)
	if err != nil {
		return value
	}
	var cloned any
	if json.Unmarshal(encoded, &cloned) != nil {
		return value
	}
	return cloned
}
