package command

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/luis-c465/fw-fanctrl/internal/config"
	"github.com/luis-c465/fw-fanctrl/internal/controller"
	"github.com/luis-c465/fw-fanctrl/internal/dto"
	"github.com/luis-c465/fw-fanctrl/internal/hardware"
	"github.com/luis-c465/fw-fanctrl/resources"
)

type mockHardwareController struct {
	mu sync.Mutex

	temperature float64
	onAC        bool
	speed       int
}

func (m *mockHardwareController) GetTemperature() (float64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.temperature, nil
}

func (m *mockHardwareController) GetTemperatures() ([]hardware.SensorReading, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	return []hardware.SensorReading{{Name: "cpu@4c", Index: 0, TempC: m.temperature, Present: true}}, nil
}

func (m *mockHardwareController) SetSpeed(speed int) error {
	m.mu.Lock()
	m.speed = speed
	m.mu.Unlock()

	return nil
}

func (m *mockHardwareController) Pause() error {
	return nil
}

func (m *mockHardwareController) Resume() error {
	return nil
}

func (m *mockHardwareController) IsOnAC() (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.onAC, nil
}

func TestHandleCommandUseAndPrintCurrent(t *testing.T) {
	handler := newTestHandler(t)

	result, err := handler.HandleCommand(UseCommand, map[string]string{"strategy": "medium"}, dto.Natural)
	if err != nil {
		t.Fatalf("use command failed: %v", err)
	}

	if result != "Strategy in use: 'medium'\nDefault: False" {
		t.Fatalf("unexpected use output: %q", result)
	}

	result, err = handler.HandleCommand(PrintCommand, map[string]string{"print_selection": "current"}, dto.Natural)
	if err != nil {
		t.Fatalf("print current command failed: %v", err)
	}

	if result != "Strategy in use: 'medium'\nDefault: False" {
		t.Fatalf("unexpected print current output: %q", result)
	}
}

func TestHandleCommandPrintAll(t *testing.T) {
	handler := newTestHandler(t)

	result, err := handler.HandleCommand(PrintCommand, map[string]string{"print_selection": "all"}, dto.Natural)
	if err != nil {
		t.Fatalf("print all command failed: %v", err)
	}

	if !strings.Contains(result, "Strategy: 'lazy'") {
		t.Fatalf("print all output missing strategy: %q", result)
	}

	if !strings.Contains(result, "Default: True") {
		t.Fatalf("print all output missing default flag: %q", result)
	}
}

func TestHandleCommandSetConfig(t *testing.T) {
	handler := newTestHandler(t)

	providedConfig := `{"defaultStrategy":"medium","strategyOnDischarging":"","strategies":{"medium":{"fanSpeedUpdateFrequency":5,"movingAverageInterval":20,"speedCurve":[{"temp":0,"speed":10},{"temp":85,"speed":100}]}}}`

	result, err := handler.HandleCommand(SetConfigCommand, map[string]string{"provided_config": providedConfig}, dto.JSON)
	if err != nil {
		t.Fatalf("set_config command failed: %v", err)
	}

	if !strings.Contains(result, `"status":"success"`) {
		t.Fatalf("unexpected set_config json output: %q", result)
	}

	if !strings.Contains(result, `"strategy":"medium"`) {
		t.Fatalf("set_config output should use medium strategy: %q", result)
	}
}

func newTestHandler(t *testing.T) *CommandHandler {
	t.Helper()

	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(configPath, resources.DefaultConfigJSON, 0o644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := config.NewConfiguration(configPath)
	if err != nil {
		t.Fatalf("failed to load test config: %v", err)
	}

	hw := &mockHardwareController{temperature: 55, onAC: true}
	fanController := controller.NewFanController(hw, cfg, "")

	return NewCommandHandler(fanController)
}
