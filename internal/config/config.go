package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/kaptinlin/jsonschema"
	"github.com/luis-c465/fw-fanctrl/resources"
)

type RawConfig struct {
	Schema                string                    `json:"$schema"`
	DefaultStrategy       string                    `json:"defaultStrategy"`
	StrategyOnDischarging string                    `json:"strategyOnDischarging"`
	Strategies            map[string]StrategyParams `json:"strategies"`
}

type Configuration struct {
	Path string
	Data RawConfig
}

var (
	compiledSchemaOnce sync.Once
	compiledSchema     *jsonschema.Schema
	compiledSchemaErr  error

	defaultConfigOnce sync.Once
	defaultConfig     RawConfig
	defaultConfigErr  error
)

func NewConfiguration(path string) (*Configuration, error) {
	c := &Configuration{Path: path}
	if err := c.Reload(); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *Configuration) Parse(rawJSON []byte) (RawConfig, error) {
	var parsedMap map[string]any
	if err := json.Unmarshal(rawJSON, &parsedMap); err != nil {
		return RawConfig{}, ConfigurationParsingError{Message: fmt.Sprintf("invalid JSON: %v", err)}
	}

	if _, hasSchema := parsedMap["$schema"]; !hasSchema {
		defaultCfg, err := getDefaultConfig()
		if err != nil {
			return RawConfig{}, ConfigurationParsingError{Message: fmt.Sprintf("failed to load embedded default config: %v", err)}
		}

		parsedMap["$schema"] = defaultCfg.Schema
	}

	normalizedJSON, err := json.Marshal(parsedMap)
	if err != nil {
		return RawConfig{}, ConfigurationParsingError{Message: fmt.Sprintf("failed to normalize config JSON: %v", err)}
	}

	schema, err := getCompiledSchema()
	if err != nil {
		return RawConfig{}, ConfigurationParsingError{Message: fmt.Sprintf("failed to compile embedded schema: %v", err)}
	}

	validation := schema.ValidateJSON(normalizedJSON)
	if !validation.IsValid() {
		return RawConfig{}, ConfigurationParsingError{Message: fmt.Sprintf("schema validation failed: %s", validation.Error())}
	}

	var cfg RawConfig
	if err := json.Unmarshal(normalizedJSON, &cfg); err != nil {
		return RawConfig{}, ConfigurationParsingError{Message: fmt.Sprintf("failed to parse validated config: %v", err)}
	}

	if _, ok := cfg.Strategies[cfg.DefaultStrategy]; !ok {
		return RawConfig{}, ConfigurationParsingError{Message: fmt.Sprintf("defaultStrategy %q does not exist in strategies", cfg.DefaultStrategy)}
	}

	if cfg.StrategyOnDischarging != "" {
		if _, ok := cfg.Strategies[cfg.StrategyOnDischarging]; !ok {
			return RawConfig{}, ConfigurationParsingError{Message: fmt.Sprintf("strategyOnDischarging %q does not exist in strategies", cfg.StrategyOnDischarging)}
		}
	}

	return cfg, nil
}

func (c *Configuration) Reload() error {
	if _, err := os.Stat(c.Path); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}

		if err := os.MkdirAll(filepath.Dir(c.Path), 0o755); err != nil {
			return err
		}

		if err := os.WriteFile(c.Path, resources.DefaultConfigJSON, 0o644); err != nil {
			return err
		}
	}

	rawJSON, err := os.ReadFile(c.Path)
	if err != nil {
		return err
	}

	cfg, err := c.Parse(rawJSON)
	if err != nil {
		return err
	}

	c.Data = cfg
	return nil
}

func (c *Configuration) Save() error {
	payload, err := json.MarshalIndent(c.Data, "", "    ")
	if err != nil {
		return err
	}

	payload = append(payload, '\n')

	if err := os.MkdirAll(filepath.Dir(c.Path), 0o755); err != nil {
		return err
	}

	return os.WriteFile(c.Path, payload, 0o644)
}

func (c *Configuration) GetStrategies() []string {
	strategies := make([]string, 0, len(c.Data.Strategies))
	for strategy := range c.Data.Strategies {
		strategies = append(strategies, strategy)
	}
	sort.Strings(strategies)

	return strategies
}

func (c *Configuration) GetStrategy(name string) (Strategy, error) {
	strategyName := name

	switch name {
	case "defaultStrategy":
		strategyName = c.Data.DefaultStrategy
	case "strategyOnDischarging":
		strategyName = c.Data.StrategyOnDischarging
		if strategyName == "" {
			strategyName = c.Data.DefaultStrategy
		}
	}

	params, ok := c.Data.Strategies[strategyName]
	if !ok {
		return Strategy{}, InvalidStrategyError{StrategyName: strategyName}
	}

	return NewStrategy(strategyName, params), nil
}

func (c *Configuration) GetDefaultStrategy() (Strategy, error) {
	return c.GetStrategy("defaultStrategy")
}

func (c *Configuration) GetDischargingStrategy() (Strategy, error) {
	return c.GetStrategy("strategyOnDischarging")
}

func getCompiledSchema() (*jsonschema.Schema, error) {
	compiledSchemaOnce.Do(func() {
		compiler := jsonschema.NewCompiler()
		compiledSchema, compiledSchemaErr = compiler.Compile(resources.ConfigSchemaJSON)
	})

	return compiledSchema, compiledSchemaErr
}

func getDefaultConfig() (RawConfig, error) {
	defaultConfigOnce.Do(func() {
		defaultConfigErr = json.Unmarshal(resources.DefaultConfigJSON, &defaultConfig)
	})

	return defaultConfig, defaultConfigErr
}
