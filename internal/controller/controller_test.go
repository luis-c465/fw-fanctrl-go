package controller

import (
	"math"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/TamtamHero/fw-fanctrl/internal/config"
	"github.com/TamtamHero/fw-fanctrl/resources"
)

const floatEpsilon = 0.000001

type mockHardwareController struct {
	mu sync.Mutex

	temperature float64
	onAC        bool

	setSpeeds   []int
	pauseCalls  int
	resumeCalls int

	temperatureErr error
	setSpeedErr    error
	pauseErr       error
	resumeErr      error
	onACErr        error
}

func (m *mockHardwareController) GetTemperature() (float64, error) {
	if m.temperatureErr != nil {
		return 0, m.temperatureErr
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	return m.temperature, nil
}

func (m *mockHardwareController) SetSpeed(speed int) error {
	if m.setSpeedErr != nil {
		return m.setSpeedErr
	}

	m.mu.Lock()
	m.setSpeeds = append(m.setSpeeds, speed)
	m.mu.Unlock()

	return nil
}

func (m *mockHardwareController) Pause() error {
	if m.pauseErr != nil {
		return m.pauseErr
	}

	m.mu.Lock()
	m.pauseCalls++
	m.mu.Unlock()

	return nil
}

func (m *mockHardwareController) Resume() error {
	if m.resumeErr != nil {
		return m.resumeErr
	}

	m.mu.Lock()
	m.resumeCalls++
	m.mu.Unlock()

	return nil
}

func (m *mockHardwareController) IsOnAC() (bool, error) {
	if m.onACErr != nil {
		return false, m.onACErr
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	return m.onAC, nil
}

func (m *mockHardwareController) lastSetSpeed() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.setSpeeds) == 0 {
		return -1
	}

	return m.setSpeeds[len(m.setSpeeds)-1]
}

func TestGetMovingAverageTemperatureFallsBackToActualTemp(t *testing.T) {
	t.Parallel()

	hw := &mockHardwareController{temperature: 48.123}
	fc := newTestFanController(t, hw)

	fc.mu.Lock()
	for i := range fc.tempHistory {
		fc.tempHistory[i] = 0
	}
	fc.mu.Unlock()

	got, err := fc.GetMovingAverageTemperature(5)
	if err != nil {
		t.Fatalf("GetMovingAverageTemperature() returned error: %v", err)
	}

	want := 48.12
	if !almostEqual(got, want) {
		t.Fatalf("unexpected moving average: got %.2f, want %.2f", got, want)
	}
}

func TestGetMovingAverageTemperatureUsesLastNNonZeroValues(t *testing.T) {
	t.Parallel()

	hw := &mockHardwareController{temperature: 30}
	fc := newTestFanController(t, hw)

	fc.mu.Lock()
	fc.tempHistory = []float64{0, 10, 20, 0, 30, 40}
	fc.mu.Unlock()

	got, err := fc.GetMovingAverageTemperature(3)
	if err != nil {
		t.Fatalf("GetMovingAverageTemperature() returned error: %v", err)
	}

	want := 30.0
	if !almostEqual(got, want) {
		t.Fatalf("unexpected moving average: got %.2f, want %.2f", got, want)
	}
}

func TestGetEffectiveTemperatureReturnsMinimum(t *testing.T) {
	t.Parallel()

	hw := &mockHardwareController{temperature: 70}
	fc := newTestFanController(t, hw)

	fc.mu.Lock()
	fc.tempHistory = []float64{50, 60}
	fc.mu.Unlock()

	got, err := fc.GetEffectiveTemperature(60, 2)
	if err != nil {
		t.Fatalf("GetEffectiveTemperature() returned error: %v", err)
	}

	want := 55.0
	if !almostEqual(got, want) {
		t.Fatalf("unexpected effective temp: got %.2f, want %.2f", got, want)
	}
}

func TestAdaptSpeedLazyStrategyCurve(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		temp     float64
		expected int
	}{
		{name: "at 0C", temp: 0, expected: 15},
		{name: "at 50C", temp: 50, expected: 15},
		{name: "at 57.5C", temp: 57.5, expected: 20},
		{name: "at 65C", temp: 65, expected: 25},
		{name: "at 85C", temp: 85, expected: 100},
		{name: "at 100C", temp: 100, expected: 100},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			hw := &mockHardwareController{temperature: tc.temp, onAC: true}
			fc := newTestFanController(t, hw)

			if err := fc.AdaptSpeed(tc.temp); err != nil {
				t.Fatalf("AdaptSpeed() returned error: %v", err)
			}

			if got := hw.lastSetSpeed(); got != tc.expected {
				t.Fatalf("unexpected speed: got %d, want %d", got, tc.expected)
			}
		})
	}
}

func TestOverwriteAndClearStrategy(t *testing.T) {
	t.Parallel()

	hw := &mockHardwareController{temperature: 42, onAC: true}
	fc := newTestFanController(t, hw)

	if err := fc.OverwriteStrategy("medium"); err != nil {
		t.Fatalf("OverwriteStrategy() returned error: %v", err)
	}

	strategy, err := fc.GetCurrentStrategy()
	if err != nil {
		t.Fatalf("GetCurrentStrategy() returned error: %v", err)
	}

	if strategy.Name != "medium" {
		t.Fatalf("expected overwritten strategy 'medium', got %q", strategy.Name)
	}

	fc.ClearOverwrittenStrategy()

	strategy, err = fc.GetCurrentStrategy()
	if err != nil {
		t.Fatalf("GetCurrentStrategy() after clear returned error: %v", err)
	}

	if strategy.Name != "lazy" {
		t.Fatalf("expected default strategy 'lazy' after clear, got %q", strategy.Name)
	}
}

func TestPauseResumeToggleActiveState(t *testing.T) {
	t.Parallel()

	hw := &mockHardwareController{temperature: 40}
	fc := newTestFanController(t, hw)

	if err := fc.Pause(); err != nil {
		t.Fatalf("Pause() returned error: %v", err)
	}

	if fc.IsActive() {
		t.Fatal("expected controller to be inactive after Pause()")
	}

	if err := fc.Resume(); err != nil {
		t.Fatalf("Resume() returned error: %v", err)
	}

	if !fc.IsActive() {
		t.Fatal("expected controller to be active after Resume()")
	}

	hw.mu.Lock()
	pauseCalls := hw.pauseCalls
	resumeCalls := hw.resumeCalls
	hw.mu.Unlock()

	if pauseCalls != 1 {
		t.Fatalf("expected hardware Pause() to be called once, got %d", pauseCalls)
	}

	if resumeCalls != 1 {
		t.Fatalf("expected hardware Resume() to be called once, got %d", resumeCalls)
	}
}

func newTestFanController(t *testing.T, hw *mockHardwareController) *FanController {
	t.Helper()

	cfgPath := filepath.Join(t.TempDir(), "config.json")
	if err := writeFile(cfgPath, resources.DefaultConfigJSON); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := config.NewConfiguration(cfgPath)
	if err != nil {
		t.Fatalf("failed to build configuration: %v", err)
	}

	return NewFanController(hw, cfg, "")
}

func writeFile(path string, content []byte) error {
	return os.WriteFile(path, content, 0o644)
}

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < floatEpsilon
}
