package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/TamtamHero/fw-fanctrl/resources"
)

func TestParseEmbeddedDefaultConfig(t *testing.T) {
	t.Parallel()

	cfg := &Configuration{}
	parsed, err := cfg.Parse(resources.DefaultConfigJSON)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	if len(parsed.Strategies) != 7 {
		t.Fatalf("expected 7 strategies, got %d", len(parsed.Strategies))
	}
}

func TestGetDefaultStrategyReturnsLazy(t *testing.T) {
	t.Parallel()

	cfg := &Configuration{}
	parsed, err := cfg.Parse(resources.DefaultConfigJSON)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}
	cfg.Data = parsed

	strategy, err := cfg.GetDefaultStrategy()
	if err != nil {
		t.Fatalf("GetDefaultStrategy() returned error: %v", err)
	}

	if strategy.Name != "lazy" {
		t.Fatalf("expected default strategy 'lazy', got %q", strategy.Name)
	}
}

func TestGetDischargingStrategyFallsBackToDefault(t *testing.T) {
	t.Parallel()

	cfg := &Configuration{}
	parsed, err := cfg.Parse(resources.DefaultConfigJSON)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}
	cfg.Data = parsed

	strategy, err := cfg.GetDischargingStrategy()
	if err != nil {
		t.Fatalf("GetDischargingStrategy() returned error: %v", err)
	}

	if strategy.Name != "lazy" {
		t.Fatalf("expected fallback strategy 'lazy', got %q", strategy.Name)
	}
}

func TestGetStrategyInvalidName(t *testing.T) {
	t.Parallel()

	cfg := &Configuration{}
	parsed, err := cfg.Parse(resources.DefaultConfigJSON)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}
	cfg.Data = parsed

	_, err = cfg.GetStrategy("does-not-exist")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var invalidStrategyErr InvalidStrategyError
	if !errors.As(err, &invalidStrategyErr) {
		t.Fatalf("expected InvalidStrategyError, got %T", err)
	}
}

func TestParseInvalidJSON(t *testing.T) {
	t.Parallel()

	cfg := &Configuration{}
	_, err := cfg.Parse([]byte("{invalid"))
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}

	var parseErr ConfigurationParsingError
	if !errors.As(err, &parseErr) {
		t.Fatalf("expected ConfigurationParsingError, got %T", err)
	}
}

func TestParseMissingRequiredFields(t *testing.T) {
	t.Parallel()

	cfg := &Configuration{}
	_, err := cfg.Parse([]byte(`{"defaultStrategy":"lazy","strategies":{}}`))
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}

	var parseErr ConfigurationParsingError
	if !errors.As(err, &parseErr) {
		t.Fatalf("expected ConfigurationParsingError, got %T", err)
	}
}

func TestParseInvalidDefaultStrategyReference(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
		"$schema": "./config.schema.json",
		"defaultStrategy": "does-not-exist",
		"strategyOnDischarging": "",
		"strategies": {
			"lazy": {
				"fanSpeedUpdateFrequency": 5,
				"movingAverageInterval": 20,
				"speedCurve": [{"temp": 0, "speed": 10}]
			}
		}
	}`)

	cfg := &Configuration{}
	_, err := cfg.Parse(payload)
	if err == nil {
		t.Fatal("expected invalid default strategy error, got nil")
	}

	var parseErr ConfigurationParsingError
	if !errors.As(err, &parseErr) {
		t.Fatalf("expected ConfigurationParsingError, got %T", err)
	}
}

func TestParseInvalidDischargingStrategyReference(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
		"$schema": "./config.schema.json",
		"defaultStrategy": "lazy",
		"strategyOnDischarging": "does-not-exist",
		"strategies": {
			"lazy": {
				"fanSpeedUpdateFrequency": 5,
				"movingAverageInterval": 20,
				"speedCurve": [{"temp": 0, "speed": 10}]
			}
		}
	}`)

	cfg := &Configuration{}
	_, err := cfg.Parse(payload)
	if err == nil {
		t.Fatal("expected invalid discharging strategy error, got nil")
	}

	var parseErr ConfigurationParsingError
	if !errors.As(err, &parseErr) {
		t.Fatalf("expected ConfigurationParsingError, got %T", err)
	}
}

func TestReloadCreatesDefaultFileWhenMissing(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "fw-fanctrl", "config.json")
	cfg := &Configuration{Path: configPath}

	if err := cfg.Reload(); err != nil {
		t.Fatalf("Reload() returned error: %v", err)
	}

	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("expected config file to exist at %q: %v", configPath, err)
	}

	if len(cfg.Data.Strategies) != 7 {
		t.Fatalf("expected 7 strategies after reload, got %d", len(cfg.Data.Strategies))
	}
}

func TestSaveWritesParsableJSON(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(configPath, resources.DefaultConfigJSON, 0o644); err != nil {
		t.Fatalf("failed to seed config file: %v", err)
	}

	cfg, err := NewConfiguration(configPath)
	if err != nil {
		t.Fatalf("NewConfiguration() returned error: %v", err)
	}

	cfg.Data.DefaultStrategy = "medium"

	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read saved config: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("saved file is not valid JSON: %v", err)
	}

	if parsed["defaultStrategy"] != "medium" {
		t.Fatalf("expected saved defaultStrategy to be 'medium', got %v", parsed["defaultStrategy"])
	}

	reloaded, err := NewConfiguration(configPath)
	if err != nil {
		t.Fatalf("reloading saved config failed: %v", err)
	}

	if reloaded.Data.DefaultStrategy != "medium" {
		t.Fatalf("expected reloaded defaultStrategy to be 'medium', got %q", reloaded.Data.DefaultStrategy)
	}
}
