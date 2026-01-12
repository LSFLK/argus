package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEnums_DefaultValues(t *testing.T) {
	// Test loading with non-existent file (should return defaults)
	enums, err := LoadEnums("/nonexistent/path/enums.yaml")
	if err != nil {
		t.Fatalf("Expected no error for non-existent file, got: %v", err)
	}

	if enums == nil {
		t.Fatal("Expected default enums, got nil")
	}

	// Verify default values are present
	if len(enums.EventTypes) == 0 {
		t.Error("Expected default event types")
	}
	if len(enums.EventActions) == 0 {
		t.Error("Expected default event actions")
	}
	if len(enums.ActorTypes) == 0 {
		t.Error("Expected default actor types")
	}
	if len(enums.TargetTypes) == 0 {
		t.Error("Expected default target types")
	}
}

func TestLoadEnums_ValidYAML(t *testing.T) {
	// Create a temporary YAML file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "enums.yaml")
	configContent := `enums:
  eventTypes:
    - MANAGEMENT_EVENT
    - USER_MANAGEMENT
  eventActions:
    - CREATE
    - READ
  actorTypes:
    - SERVICE
    - ADMIN
  targetTypes:
    - SERVICE
    - RESOURCE
`

	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	enums, err := LoadEnums(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify loaded values (YAML values should be present, plus defaults are merged)
	// YAML has: MANAGEMENT_EVENT, USER_MANAGEMENT
	// Defaults have: MANAGEMENT_EVENT, USER_MANAGEMENT, DATA_FETCH
	// Merged should have all unique values
	if len(enums.EventTypes) < 2 {
		t.Errorf("Expected at least 2 event types from YAML, got %d", len(enums.EventTypes))
	}
	// Verify YAML values are present
	hasManagementEvent := false
	hasUserManagement := false
	for _, et := range enums.EventTypes {
		if et == "MANAGEMENT_EVENT" {
			hasManagementEvent = true
		}
		if et == "USER_MANAGEMENT" {
			hasUserManagement = true
		}
	}
	if !hasManagementEvent {
		t.Error("Expected MANAGEMENT_EVENT to be in event types")
	}
	if !hasUserManagement {
		t.Error("Expected USER_MANAGEMENT to be in event types")
	}
	// Verify default event types are also merged in
	hasDataFetch := false
	for _, et := range enums.EventTypes {
		if et == "DATA_FETCH" {
			hasDataFetch = true
		}
	}
	if !hasDataFetch {
		t.Error("Expected DATA_FETCH to be merged from defaults")
	}
}

func TestAuditEnums_Validation(t *testing.T) {
	enums := &AuditEnums{
		EventTypes:   []string{"MANAGEMENT_EVENT", "USER_MANAGEMENT"},
		EventActions: []string{"CREATE", "READ"},
		ActorTypes:   []string{"SERVICE", "ADMIN"},
		TargetTypes:  []string{"SERVICE", "RESOURCE"},
	}
	// Initialize maps (normally done by LoadEnums)
	enums.InitializeMaps()

	// Test valid values
	if !enums.IsValidEventType("MANAGEMENT_EVENT") {
		t.Error("MANAGEMENT_EVENT should be valid")
	}
	if !enums.IsValidEventAction("CREATE") {
		t.Error("CREATE should be valid")
	}
	if !enums.IsValidActorType("SERVICE") {
		t.Error("SERVICE should be valid")
	}
	if !enums.IsValidTargetType("RESOURCE") {
		t.Error("RESOURCE should be valid")
	}

	// Test invalid values
	if enums.IsValidEventType("INVALID") {
		t.Error("INVALID should not be valid")
	}
	if enums.IsValidEventAction("INVALID") {
		t.Error("INVALID should not be valid")
	}
	if enums.IsValidActorType("INVALID") {
		t.Error("INVALID should not be valid")
	}
	if enums.IsValidTargetType("INVALID") {
		t.Error("INVALID should not be valid")
	}

	// Test empty values (should be allowed for nullable fields)
	if !enums.IsValidEventType("") {
		t.Error("Empty event type should be valid (nullable)")
	}
	if !enums.IsValidEventAction("") {
		t.Error("Empty event action should be valid (nullable)")
	}
}
