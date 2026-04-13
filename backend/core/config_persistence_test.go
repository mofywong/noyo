package core

import (
	"encoding/json"
	"noyo/core/platform"
	"noyo/core/store"
	"noyo/core/types"
	"reflect"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type TestProvider struct {
	Name string `yaml:"name" json:"name"`
	ID   int    `yaml:"id" json:"id"`
}

type TestConfig struct {
	Providers []TestProvider `yaml:"providers" json:"providers"`
}

type MockPlugin struct {
	platform.BasePlatformPlugin
	Config TestConfig
}

func (p *MockPlugin) GetMeta() *types.PluginMeta {
	return &types.PluginMeta{Name: "mock_plugin"}
}
func (p *MockPlugin) IsEnabled() bool { return true }
func (p *MockPlugin) Start() error    { return nil }
func (p *MockPlugin) Stop() error     { return nil }

func setupTestDB() {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	store.DB = db
	db.AutoMigrate(&store.PluginModel{})
}

func TestSetReflectValue_SliceOfStructs(t *testing.T) {
	// Target struct
	cfg := TestConfig{}

	// Value to set (simulating JSON unmarshal result: []interface{} containing map[string]interface{})
	input := []interface{}{
		map[string]interface{}{
			"Name": "test1",
			"ID":   1,
		},
		map[string]interface{}{
			"Name": "test2",
			"ID":   2,
		},
	}

	// Get reflect.Value of the field
	val := reflect.ValueOf(&cfg).Elem()
	field := val.FieldByName("Providers")

	// Call the function
	setReflectValue(field, input)

	// Check result
	if len(cfg.Providers) != 2 {
		t.Errorf("Expected 2 providers, got %d", len(cfg.Providers))
	}
	if cfg.Providers[0].Name != "test1" {
		t.Errorf("Expected provider 1 name 'test1', got '%s'", cfg.Providers[0].Name)
	}
}

func TestUpdatePluginConfig_FullFlow(t *testing.T) {
	setupTestDB()

	plugin := &MockPlugin{}

	// Create update map
	newConfig := map[string]interface{}{
		"providers": []interface{}{
			map[string]interface{}{
				"name": "p1",
				"id":   100,
			},
		},
	}

	// Run UpdatePluginConfig
	err := UpdatePluginConfig(plugin, newConfig)
	if err != nil {
		t.Fatalf("UpdatePluginConfig failed: %v", err)
	}

	// 1. Verify struct in memory is updated
	if len(plugin.Config.Providers) != 1 {
		t.Errorf("Expected 1 provider in memory, got %d", len(plugin.Config.Providers))
	} else {
		if plugin.Config.Providers[0].Name != "p1" {
			t.Errorf("Expected name 'p1', got '%s'", plugin.Config.Providers[0].Name)
		}
	}

	// 2. Verify DB is updated
	model, err := store.GetPlugin("mock_plugin")
	if err != nil {
		t.Fatalf("Failed to get plugin from DB: %v", err)
	}
	if model == nil {
		t.Fatal("Plugin not found in DB")
	}

	// Verify JSON in DB
	var dbConfig TestConfig
	if err := json.Unmarshal([]byte(model.Config), &dbConfig); err != nil {
		t.Fatalf("Failed to unmarshal config from DB: %v", err)
	}

	if len(dbConfig.Providers) != 1 {
		t.Errorf("Expected 1 provider in DB, got %d", len(dbConfig.Providers))
	} else {
		if dbConfig.Providers[0].Name != "p1" {
			t.Errorf("Expected name 'p1' in DB, got '%s'", dbConfig.Providers[0].Name)
		}
	}
}
