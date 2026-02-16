package controller

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

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

	if speed := fc.GetSpeed(); speed != 0 {
		t.Fatalf("expected initial speed to remain 0, got %d", speed)
	}
}

func TestGetCurrentStrategyUsesDischargingWhenOnBattery(t *testing.T) {
	hw := &mockHardwareController{temperature: 42, onAC: false}
	fc := newTestFanController(t, hw)

	fc.mu.Lock()
	fc.config.Data.StrategyOnDischarging = "medium"
	fc.mu.Unlock()

	strategy, err := fc.GetCurrentStrategy()
	if err != nil {
		t.Fatalf("GetCurrentStrategy() returned error: %v", err)
	}

	if strategy.Name != "medium" {
		t.Fatalf("expected medium strategy on battery, got %q", strategy.Name)
	}
}

func TestIsDefaultStrategyInUseTracksOverwrite(t *testing.T) {
	hw := &mockHardwareController{temperature: 42, onAC: true}
	fc := newTestFanController(t, hw)

	if !fc.IsDefaultStrategyInUse() {
		t.Fatal("expected default strategy to be in use initially")
	}

	if err := fc.OverwriteStrategy("medium"); err != nil {
		t.Fatalf("OverwriteStrategy() returned error: %v", err)
	}

	if fc.IsDefaultStrategyInUse() {
		t.Fatal("expected default strategy to be false after overwrite")
	}

	fc.ClearOverwrittenStrategy()
	if !fc.IsDefaultStrategyInUse() {
		t.Fatal("expected default strategy to be in use after clear")
	}
}

func TestGetStrategiesReturnsSortedCopy(t *testing.T) {
	hw := &mockHardwareController{temperature: 42, onAC: true}
	fc := newTestFanController(t, hw)

	strategies := fc.GetStrategies()
	if len(strategies) != 7 {
		t.Fatalf("expected 7 strategies, got %d", len(strategies))
	}

	if strategies[0] != "aeolus" || strategies[len(strategies)-1] != "very-agile" {
		t.Fatalf("unexpected sorted strategies: %v", strategies)
	}

	strategies[0] = "mutated"
	again := fc.GetStrategies()
	if again[0] != "aeolus" {
		t.Fatalf("expected GetStrategies() to return a copy, got %v", again)
	}
}

func TestReloadConfigurationKeepsOverwrittenStrategy(t *testing.T) {
	hw := &mockHardwareController{temperature: 42, onAC: true}
	fc := newTestFanController(t, hw)

	if err := fc.OverwriteStrategy("medium"); err != nil {
		t.Fatalf("OverwriteStrategy() returned error: %v", err)
	}

	if err := fc.ReloadConfiguration(); err != nil {
		t.Fatalf("ReloadConfiguration() returned error: %v", err)
	}

	strategy, err := fc.GetCurrentStrategy()
	if err != nil {
		t.Fatalf("GetCurrentStrategy() returned error: %v", err)
	}

	if strategy.Name != "medium" {
		t.Fatalf("expected overwritten strategy to persist, got %q", strategy.Name)
	}
}

func TestSetConfigurationUpdatesDataAndPersists(t *testing.T) {
	hw := &mockHardwareController{temperature: 42, onAC: true}
	fc := newTestFanController(t, hw)

	if err := fc.OverwriteStrategy("medium"); err != nil {
		t.Fatalf("OverwriteStrategy() returned error: %v", err)
	}

	provided := []byte(`{
		"$schema": "./config.schema.json",
		"defaultStrategy": "agile",
		"strategyOnDischarging": "",
		"strategies": {
			"agile": {
				"fanSpeedUpdateFrequency": 3,
				"movingAverageInterval": 10,
				"speedCurve": [{"temp": 0, "speed": 15}, {"temp": 85, "speed": 100}]
			},
			"medium": {
				"fanSpeedUpdateFrequency": 5,
				"movingAverageInterval": 20,
				"speedCurve": [{"temp": 0, "speed": 10}, {"temp": 85, "speed": 100}]
			}
		}
	}`)

	if err := fc.SetConfiguration(provided); err != nil {
		t.Fatalf("SetConfiguration() returned error: %v", err)
	}

	fc.mu.Lock()
	if fc.config.Data.DefaultStrategy != "agile" {
		fc.mu.Unlock()
		t.Fatalf("expected defaultStrategy to be agile, got %q", fc.config.Data.DefaultStrategy)
	}
	configPath := fc.config.Path
	fc.mu.Unlock()

	strategy, err := fc.GetCurrentStrategy()
	if err != nil {
		t.Fatalf("GetCurrentStrategy() returned error: %v", err)
	}

	if strategy.Name != "medium" {
		t.Fatalf("expected overwritten strategy to persist after SetConfiguration, got %q", strategy.Name)
	}

	written, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read persisted config: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(written, &payload); err != nil {
		t.Fatalf("persisted config is invalid JSON: %v", err)
	}

	if payload["defaultStrategy"] != "agile" {
		t.Fatalf("expected persisted defaultStrategy agile, got %v", payload["defaultStrategy"])
	}
}

func TestConfigurationForOutputIncludesPathAndData(t *testing.T) {
	hw := &mockHardwareController{temperature: 42, onAC: true}
	fc := newTestFanController(t, hw)

	output := fc.ConfigurationForOutput()
	pathValue, ok := output["path"].(string)
	if !ok || pathValue == "" {
		t.Fatalf("expected non-empty path string, got %T (%v)", output["path"], output["path"])
	}

	data, ok := output["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data map, got %T", output["data"])
	}

	if data["defaultStrategy"] != "lazy" {
		t.Fatalf("expected defaultStrategy lazy, got %v", data["defaultStrategy"])
	}
}

func TestStatusSnapshotIncludesComputedTemperatures(t *testing.T) {
	hw := &mockHardwareController{temperature: 60, onAC: true}
	fc := newTestFanController(t, hw)

	fc.mu.Lock()
	fc.speed = 25
	fc.tempHistory = []float64{40, 50, 60}
	fc.mu.Unlock()

	snapshot, err := fc.StatusSnapshot()
	if err != nil {
		t.Fatalf("StatusSnapshot() returned error: %v", err)
	}

	if snapshot.Strategy != "lazy" {
		t.Fatalf("expected strategy lazy, got %q", snapshot.Strategy)
	}

	if !snapshot.Default {
		t.Fatal("expected default strategy flag to be true")
	}

	if snapshot.Speed != 25 {
		t.Fatalf("expected speed 25, got %d", snapshot.Speed)
	}

	if !almostEqual(snapshot.Temperature, 60) {
		t.Fatalf("expected temperature 60, got %.2f", snapshot.Temperature)
	}

	if !almostEqual(snapshot.MovingAverageTemperature, 50) {
		t.Fatalf("expected moving average 50, got %.2f", snapshot.MovingAverageTemperature)
	}

	if !almostEqual(snapshot.EffectiveTemperature, 50) {
		t.Fatalf("expected effective temperature 50, got %.2f", snapshot.EffectiveTemperature)
	}

	if !snapshot.Active {
		t.Fatal("expected active to be true")
	}
}

func TestAdaptSpeedDoesNotCallHardwareWhenInactive(t *testing.T) {
	hw := &mockHardwareController{temperature: 70, onAC: true}
	fc := newTestFanController(t, hw)

	if err := fc.Pause(); err != nil {
		t.Fatalf("Pause() returned error: %v", err)
	}

	if err := fc.AdaptSpeed(70); err != nil {
		t.Fatalf("AdaptSpeed() returned error: %v", err)
	}

	if got := hw.lastSetSpeed(); got != -1 {
		t.Fatalf("expected no speed set while inactive, got %d", got)
	}
}

func TestPushTemperatureMaintainsHistoryLimit(t *testing.T) {
	hw := &mockHardwareController{temperature: 42, onAC: true}
	fc := newTestFanController(t, hw)

	fc.mu.Lock()
	fc.tempHistory = nil
	fc.mu.Unlock()

	for i := 0; i < tempHistoryLimit+10; i++ {
		fc.pushTemperature(float64(i))
	}

	fc.mu.Lock()
	defer fc.mu.Unlock()

	if len(fc.tempHistory) != tempHistoryLimit {
		t.Fatalf("expected temp history length %d, got %d", tempHistoryLimit, len(fc.tempHistory))
	}

	if !almostEqual(fc.tempHistory[0], 10) {
		t.Fatalf("expected oldest value 10, got %.2f", fc.tempHistory[0])
	}

	if !almostEqual(fc.tempHistory[len(fc.tempHistory)-1], 109) {
		t.Fatalf("expected latest value 109, got %.2f", fc.tempHistory[len(fc.tempHistory)-1])
	}
}

func TestCheckShutdownWithoutSignal(t *testing.T) {
	hw := &mockHardwareController{temperature: 42, onAC: true}
	fc := newTestFanController(t, hw)

	shutdown, err := fc.checkShutdown(make(chan os.Signal, 1))
	if err != nil {
		t.Fatalf("checkShutdown() returned error: %v", err)
	}

	if shutdown {
		t.Fatal("expected shutdown to be false when no signal is received")
	}
}

func TestCheckShutdownWithSignalPausesController(t *testing.T) {
	hw := &mockHardwareController{temperature: 42, onAC: true}
	fc := newTestFanController(t, hw)

	signalCh := make(chan os.Signal, 1)
	signalCh <- os.Interrupt

	shutdown, err := fc.checkShutdown(signalCh)
	if err != nil {
		t.Fatalf("checkShutdown() returned error: %v", err)
	}

	if !shutdown {
		t.Fatal("expected shutdown to be true when signal is received")
	}

	if fc.IsActive() {
		t.Fatal("expected controller to be inactive after shutdown")
	}
}

func TestSleepOrShutdownTimeoutPath(t *testing.T) {
	hw := &mockHardwareController{temperature: 42, onAC: true}
	fc := newTestFanController(t, hw)

	shutdown, err := fc.sleepOrShutdown(1*time.Millisecond, make(chan os.Signal, 1))
	if err != nil {
		t.Fatalf("sleepOrShutdown() returned error: %v", err)
	}

	if shutdown {
		t.Fatal("expected shutdown to be false when timeout elapses")
	}
}

func TestSleepOrShutdownSignalPath(t *testing.T) {
	hw := &mockHardwareController{temperature: 42, onAC: true}
	fc := newTestFanController(t, hw)

	signalCh := make(chan os.Signal, 1)
	signalCh <- os.Interrupt

	shutdown, err := fc.sleepOrShutdown(100*time.Millisecond, signalCh)
	if err != nil {
		t.Fatalf("sleepOrShutdown() returned error: %v", err)
	}

	if !shutdown {
		t.Fatal("expected shutdown to be true when signal is received")
	}
}

func TestRunReturnsCriticalErrorWhenTemperatureReadFails(t *testing.T) {
	hw := &mockHardwareController{temperatureErr: errors.New("sensor read failed")}
	fc := newTestFanController(t, hw)

	err := fc.Run(false)
	if err == nil {
		t.Fatal("expected Run() to return error")
	}

	if !strings.Contains(err.Error(), "critical error") {
		t.Fatalf("expected critical error message, got %q", err.Error())
	}
}

func TestRunReturnsCriticalErrorWhenSpeedCurveIsEmpty(t *testing.T) {
	hw := &mockHardwareController{temperature: 42, onAC: true}
	fc := newTestFanController(t, hw)

	fc.mu.Lock()
	lazy := fc.config.Data.Strategies["lazy"]
	lazy.SpeedCurve = nil
	fc.config.Data.Strategies["lazy"] = lazy
	fc.mu.Unlock()

	err := fc.Run(false)
	if err == nil {
		t.Fatal("expected Run() to return error")
	}

	if !strings.Contains(err.Error(), "empty speed curve") {
		t.Fatalf("expected empty speed curve error, got %q", err.Error())
	}
}

func TestRunStopsGracefullyOnSignal(t *testing.T) {
	hw := &mockHardwareController{temperature: 55, onAC: true}
	fc := newTestFanController(t, hw)

	done := make(chan error, 1)
	go func() {
		done <- fc.Run(false)
	}()

	time.Sleep(20 * time.Millisecond)
	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatalf("failed to send SIGTERM: %v", err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run() did not stop after SIGTERM")
	}

	if fc.IsActive() {
		t.Fatal("expected controller to be inactive after graceful shutdown")
	}
}

func TestRunInactiveLoopStopsOnSignal(t *testing.T) {
	hw := &mockHardwareController{temperature: 55, onAC: true}
	fc := newTestFanController(t, hw)

	if err := fc.Pause(); err != nil {
		t.Fatalf("Pause() returned error: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- fc.Run(false)
	}()

	time.Sleep(20 * time.Millisecond)
	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatalf("failed to send SIGTERM: %v", err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run() did not stop after SIGTERM")
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
