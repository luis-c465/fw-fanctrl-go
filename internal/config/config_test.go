package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/luis-c465/fw-fanctrl/resources"
)

func TestParseEmbeddedDefaultConfig(t *testing.T) {
	cfg := &Configuration{}
	parsed, err := cfg.Parse(resources.DefaultConfigJSON)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	if len(parsed.Strategies) != 8 {
		t.Fatalf("expected 8 strategies, got %d", len(parsed.Strategies))
	}

	expected := map[string]StrategyParams{
		"lazy":       {FanSpeedUpdateFrequency: 5, MovingAverageInterval: 30},
		"very-agile": {FanSpeedUpdateFrequency: 2, MovingAverageInterval: 5},
		"aeolus":     {FanSpeedUpdateFrequency: 2, MovingAverageInterval: 5},
	}

	for name, want := range expected {
		got, ok := parsed.Strategies[name]
		if !ok {
			t.Fatalf("expected strategy %q to exist", name)
		}

		if got.FanSpeedUpdateFrequency != want.FanSpeedUpdateFrequency {
			t.Fatalf("strategy %q fanSpeedUpdateFrequency: got %d, want %d", name, got.FanSpeedUpdateFrequency, want.FanSpeedUpdateFrequency)
		}

		if got.MovingAverageInterval != want.MovingAverageInterval {
			t.Fatalf("strategy %q movingAverageInterval: got %d, want %d", name, got.MovingAverageInterval, want.MovingAverageInterval)
		}
	}
}

func TestParseInjectsSchemaWhenMissing(t *testing.T) {
	cfg := &Configuration{}
	raw := []byte(`{
		"defaultStrategy": "lazy",
		"strategyOnDischarging": "",
		"strategies": {
			"lazy": {
				"fanSpeedUpdateFrequency": 5,
				"movingAverageInterval": 20,
				"speedCurve": [{"temp": 0, "speed": 15}, {"temp": 85, "speed": 100}]
			}
		}
	}`)

	parsed, err := cfg.Parse(raw)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	if parsed.Schema != "./config.schema.json" {
		t.Fatalf("expected schema to be injected, got %q", parsed.Schema)
	}
}

func TestGetDefaultStrategyReturnsLazy(t *testing.T) {
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

func TestGetDischargingStrategyReturnsConfiguredStrategy(t *testing.T) {
	cfg := &Configuration{}
	parsed, err := cfg.Parse(resources.DefaultConfigJSON)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}
	parsed.StrategyOnDischarging = "medium"
	cfg.Data = parsed

	strategy, err := cfg.GetDischargingStrategy()
	if err != nil {
		t.Fatalf("GetDischargingStrategy() returned error: %v", err)
	}

	if strategy.Name != "medium" {
		t.Fatalf("expected discharging strategy 'medium', got %q", strategy.Name)
	}
}

func TestGetStrategiesReturnsSortedNames(t *testing.T) {
	cfg := &Configuration{}
	parsed, err := cfg.Parse(resources.DefaultConfigJSON)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}
	cfg.Data = parsed

	strategies := cfg.GetStrategies()
	if len(strategies) != 8 {
		t.Fatalf("expected 8 strategies, got %d", len(strategies))
	}

	expectedOrder := []string{"aeolus", "agile", "deaf", "fw16-dual-zone", "laziest", "lazy", "medium", "very-agile"}
	for i, name := range expectedOrder {
		if strategies[i] != name {
			t.Fatalf("unexpected strategy order at index %d: got %q, want %q", i, strategies[i], name)
		}
	}
}

func TestGetStrategyInvalidName(t *testing.T) {
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
	configPath := filepath.Join(t.TempDir(), "fw-fanctrl", "config.json")
	cfg := &Configuration{Path: configPath}

	if err := cfg.Reload(); err != nil {
		t.Fatalf("Reload() returned error: %v", err)
	}

	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("expected config file to exist at %q: %v", configPath, err)
	}

	if len(cfg.Data.Strategies) != 8 {
		t.Fatalf("expected 8 strategies after reload, got %d", len(cfg.Data.Strategies))
	}
}

func TestSaveWritesParsableJSON(t *testing.T) {
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

func TestParseMultiSensorStrategy(t *testing.T) {
	cfg := &Configuration{}
	raw := []byte(`{
		"$schema": "./config.schema.json",
		"defaultStrategy": "multi",
		"strategyOnDischarging": "",
		"strategies": {
			"multi": {
				"fanSpeedUpdateFrequency": 5,
				"movingAverageInterval": 30,
				"sensorCurves": [
					{
						"name": "cpu",
						"sensors": ["cpu@4c"],
						"speedCurve": [{"temp": 0, "speed": 15}, {"temp": 85, "speed": 100}]
					}
				]
			}
		}
	}`)

	parsed, err := cfg.Parse(raw)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	strategy := NewStrategy("multi", parsed.Strategies["multi"])
	if !strategy.IsMultiSensor() {
		t.Fatal("expected parsed strategy to be multi-sensor")
	}
}

func TestParseRejectsEmptySensorCurveName(t *testing.T) {
	cfg := &Configuration{}
	raw := []byte(`{
		"$schema": "./config.schema.json",
		"defaultStrategy": "multi",
		"strategyOnDischarging": "",
		"strategies": {
			"multi": {
				"movingAverageInterval": 30,
				"sensorCurves": [
					{
						"name": "",
						"sensors": ["cpu@4c"],
						"speedCurve": [{"temp": 0, "speed": 15}]
					}
				]
			}
		}
	}`)

	_, err := cfg.Parse(raw)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
}
