package core

import (
	"fmt"
	"noyo/core/store"
	"reflect"
	"strings"

	"github.com/gogf/gf/v2/util/gconv"
)

// ConfigField describes a single configuration field
type ConfigField struct {
	Name        string              `json:"name"`        // YAML key
	Type        string              `json:"type"`        // string, int, bool, etc.
	Title       map[string]string   `json:"title"`       // Display name in different languages
	Description map[string]string   `json:"description"` // Helper text in different languages
	Value       interface{}         `json:"value"`       // Current value
	Options     []map[string]string `json:"options"`     // List of options for select type (value, label)
}

// IConfigSchemaProvider allows plugins to provide dynamic configuration schema
type IConfigSchemaProvider interface {
	GetConfigSchema() *PluginConfigSchema
}

// PluginConfigSchema describes the configuration structure of a plugin
type PluginConfigSchema struct {
	PluginName  string            `json:"plugin_name"`
	Title       map[string]string `json:"title"`       // Display Name (i18n)
	Description map[string]string `json:"description"` // Description (i18n)
	Fields      []ConfigField     `json:"fields"`
}

// GetPluginConfigSchema generates the schema from the plugin's Config struct
func GetPluginConfigSchema(plugin IManagedPlugin) *PluginConfigSchema {
	// 1. Check if plugin implements IConfigSchemaProvider for dynamic schema
	if provider, ok := plugin.(IConfigSchemaProvider); ok {
		return provider.GetConfigSchema()
	}

	meta := plugin.GetMeta()
	schema := &PluginConfigSchema{
		PluginName:  meta.Name,
		Title:       meta.Title,
		Description: meta.Description,
		Fields:      make([]ConfigField, 0),
	}

	// Add Enabled field
	schema.Fields = append(schema.Fields, ConfigField{
		Name: "enabled",
		Type: "switch",
		Title: map[string]string{
			"en": "Enable Plugin",
			"zh": "启用插件",
		},
		Description: map[string]string{
			"en": "Enable or disable this plugin",
			"zh": "启用或禁用此插件",
		},
		Value: plugin.IsEnabled(),
	})

	// Access the embedded core.Plugin to get the Config interface
	// Since IPlugin is an interface, we need to assert it or use reflection
	// However, we know all plugins embed core.Plugin, but we can't access fields directly from interface
	// We have to use reflection on the plugin instance

	val := reflect.ValueOf(plugin)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	// Find "Config" field in the specific plugin struct (e.g. SagooPlugin.Config)
	configField := val.FieldByName("Config")
	if !configField.IsValid() {
		return schema
	}

	configType := configField.Type()

	// Ensure it is a struct
	if configType.Kind() != reflect.Struct {
		return schema
	}

	// Iterate fields of the Config struct
	for i := 0; i < configType.NumField(); i++ {
		field := configType.Field(i)
		fieldVal := configField.Field(i)

		// Skip unexported fields
		if field.PkgPath != "" {
			continue
		}

		// Get YAML tag for name
		yamlTag := field.Tag.Get("yaml")
		name := strings.Split(yamlTag, ",")[0]
		if name == "" {
			name = field.Name
		}

		// Determine type
		typeName := field.Type.Name()
		if typeName == "" {
			typeName = field.Type.Kind().String()
		}

		// Get i18n tags
		title := make(map[string]string)
		desc := make(map[string]string)

		// Title
		if t := field.Tag.Get("title_en"); t != "" {
			title["en"] = t
		} else {
			title["en"] = name // Default to name
		}
		if t := field.Tag.Get("title_zh"); t != "" {
			title["zh"] = t
		} else {
			title["zh"] = name // Default to name
		}

		// Description
		if d := field.Tag.Get("desc_en"); d != "" {
			desc["en"] = d
		}
		if d := field.Tag.Get("desc_zh"); d != "" {
			desc["zh"] = d
		}

		schema.Fields = append(schema.Fields, ConfigField{
			Name:        name,
			Type:        typeName,
			Title:       title,
			Description: desc,
			Value:       fieldVal.Interface(),
		})
	}

	return schema
}

// UpdatePluginConfig updates the plugin's config from a map
func UpdatePluginConfig(plugin IManagedPlugin, newConfig map[string]interface{}) error {
	// 1. Handle Enabled status
	var enabled bool

	// Check for "enabled" or "enable"
	var enabledVal interface{}
	var found bool

	if v, ok := newConfig["enabled"]; ok {
		enabledVal = v
		found = true
	} else if v, ok := newConfig["enable"]; ok {
		enabledVal = v
		found = true
	}

	if found {
		// Handle various types
		switch v := enabledVal.(type) {
		case bool:
			enabled = v
		case string:
			lower := strings.ToLower(v)
			enabled = (lower == "true" || lower == "1" || lower == "yes" || lower == "on")
		case int:
			enabled = (v != 0)
		case float64:
			enabled = (v != 0)
		default:
			fmt.Printf("Warning: 'enabled' field has unexpected type: %T, value: %v\n", v, v)
			// Keep enabled as false if type is unknown
		}
	} else {
		// If not provided in update, retain current state
		enabled = plugin.IsEnabled()
	}

	// Update Enabled field on struct (in memory)
	val := reflect.ValueOf(plugin)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	// We need to access the Enabled field. Plugins may embed:
	// - core.Plugin (legacy)
	// - platform.BasePlatformPlugin
	// - protocol.BaseProtocolPlugin
	// All of these should have an "Enabled" field that is promoted.
	enabledField := val.FieldByName("Enabled")
	if enabledField.IsValid() && enabledField.CanSet() {
		enabledField.SetBool(enabled)
	} else {
		// Try via embedded structs
		for _, embedName := range []string{"Plugin", "BasePlatformPlugin", "BaseProtocolPlugin"} {
			embedField := val.FieldByName(embedName)
			if embedField.IsValid() {
				enabledField = embedField.FieldByName("Enabled")
				if enabledField.IsValid() && enabledField.CanSet() {
					enabledField.SetBool(enabled)
					break
				}
			}
		}
	}

	configField := val.FieldByName("Config")
	// If Config field is valid, update it
	if configField.IsValid() {
		// Iterate fields and set values
		configType := configField.Type()
		for i := 0; i < configType.NumField(); i++ {
			field := configType.Field(i)

			// Get YAML tag
			yamlTag := field.Tag.Get("yaml")
			name := strings.Split(yamlTag, ",")[0]
			if name == "" {
				name = field.Name
			}

			if newVal, ok := newConfig[name]; ok {
				fieldVal := configField.Field(i)
				if fieldVal.CanSet() {
					// Log the update attempt
					// fmt.Printf("Updating field %s with value %v (type %T)\n", name, newVal, newVal)
					setReflectValue(fieldVal, newVal)
				} else {
					fmt.Printf("Field %s cannot be set\n", name)
				}
			}
		}
	}

	// Persist Config to DB
	// We need the config object.
	var configToSave interface{}
	if configField.IsValid() {
		configToSave = configField.Interface()
	}

	if err := store.SavePlugin(plugin.GetMeta().Name, enabled, configToSave); err != nil {
		return fmt.Errorf("failed to save plugin config to db: %w", err)
	}

	return nil
}

func setReflectValue(field reflect.Value, value interface{}) {
	if value == nil {
		return
	}

	// Explicitly handle empty slice to clear the target field
	// Because gconv.Scan may skip empty source slices when scanning into existing slices
	rVal := reflect.ValueOf(value)
	if rVal.IsValid() && (rVal.Kind() == reflect.Slice || rVal.Kind() == reflect.Array) {
		if rVal.Len() == 0 && field.Kind() == reflect.Slice {
			field.Set(reflect.MakeSlice(field.Type(), 0, 0))
			return
		}
	}

	// Use gconv for powerful conversion (handles structs, slices, maps, etc.)
	if field.CanAddr() {
		if err := gconv.Scan(value, field.Addr().Interface()); err != nil {
			fmt.Printf("Failed to set field %s: %v\n", field.Type(), err)
		}
		return
	}

	val := reflect.ValueOf(value)

	fmt.Printf("Setting field type %s with value type %s\n", field.Type(), val.Type())

	// Handle type mismatch (e.g. float64 -> int, which is common with JSON)
	if field.Kind() == reflect.Int || field.Kind() == reflect.Int64 {
		if val.Kind() == reflect.Float64 {
			field.SetInt(int64(val.Float()))
			return
		}
		// Handle int -> int64 etc
		if val.CanConvert(field.Type()) {
			field.Set(val.Convert(field.Type()))
			return
		}
	}

	if field.Kind() == reflect.String {
		if val.Kind() == reflect.String {
			field.SetString(val.String())
			return
		}
	}

	if field.Kind() == reflect.Bool {
		if val.Kind() == reflect.Bool {
			field.SetBool(val.Bool())
			return
		}
	}

	if field.Type() == val.Type() {
		field.Set(val)
	} else if val.CanConvert(field.Type()) {
		field.Set(val.Convert(field.Type()))
	} else {
		fmt.Printf("Failed to set field: types incompatible %s vs %s\n", field.Type(), val.Type())
	}
}
