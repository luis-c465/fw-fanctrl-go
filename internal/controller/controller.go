package controller

import (
	"errors"
	"fmt"
	"math"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/luis-c465/fw-fanctrl/internal/config"
	"github.com/luis-c465/fw-fanctrl/internal/hardware"
)

const (
	tempHistoryLimit = 100
)

type StatusSnapshot struct {
	Strategy                 string
	Default                  bool
	Speed                    int
	Temperature              float64
	MovingAverageTemperature float64
	EffectiveTemperature     float64
	Active                   bool
	Configuration            map[string]any
}

type FanController struct {
	hardware            hardware.HardwareController
	config              *config.Configuration
	overwrittenStrategy *config.Strategy
	speed               int
	tempHistory         []float64
	tempHistoryHead     int
	active              bool
	timecount           int
	mu                  sync.Mutex
}

func NewFanController(hw hardware.HardwareController, cfg *config.Configuration, strategyName string) *FanController {
	controller := &FanController{
		hardware:        hw,
		config:          cfg,
		speed:           0,
		tempHistory:     make([]float64, tempHistoryLimit),
		tempHistoryHead: 0,
		active:          true,
		timecount:       0,
	}

	if strategyName != "" {
		if err := controller.OverwriteStrategy(strategyName); err != nil {
			controller.ClearOverwrittenStrategy()
		}
	}

	return controller
}

func (f *FanController) GetActualTemperature() (float64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.hardware.GetTemperature()
}

func (f *FanController) GetMovingAverageTemperature(timeInterval int) (float64, error) {
	f.mu.Lock()
	historyLen := len(f.tempHistory)
	head := f.tempHistoryHead
	targetNonZero := historyLen
	if timeInterval > 0 {
		targetNonZero = timeInterval
	}

	total := 0.0
	nonZeroCount := 0

	if historyLen == tempHistoryLimit {
		for i := 0; i < historyLen; i++ {
			idx := head - 1 - i
			if idx < 0 {
				idx += historyLen
			}

			temp := f.tempHistory[idx]
			if temp > 0 {
				total += temp
				nonZeroCount++
				if nonZeroCount == targetNonZero {
					break
				}
			}
		}
	} else {
		for i := historyLen - 1; i >= 0; i-- {
			temp := f.tempHistory[i]
			if temp > 0 {
				total += temp
				nonZeroCount++
				if nonZeroCount == targetNonZero {
					break
				}
			}
		}
	}
	f.mu.Unlock()

	if nonZeroCount == 0 {
		temp, err := f.GetActualTemperature()
		if err != nil {
			return 0, err
		}
		return round2(temp), nil
	}

	return round2(total / float64(nonZeroCount)), nil
}

func (f *FanController) GetEffectiveTemperature(currentTemp float64, timeInterval int) (float64, error) {
	movingAverageTemp, err := f.GetMovingAverageTemperature(timeInterval)
	if err != nil {
		return 0, err
	}

	return round2(math.Min(movingAverageTemp, currentTemp)), nil
}

func (f *FanController) AdaptSpeed(currentTemp float64) error {
	currentStrategy, err := f.GetCurrentStrategy()
	if err != nil {
		return err
	}

	if len(currentStrategy.SpeedCurve) == 0 {
		return errors.New("current strategy has empty speed curve")
	}

	effectiveTemp, err := f.GetEffectiveTemperature(currentTemp, currentStrategy.MovingAverageInterval)
	if err != nil {
		return err
	}

	minPoint := currentStrategy.SpeedCurve[0]
	maxPoint := currentStrategy.SpeedCurve[len(currentStrategy.SpeedCurve)-1]

	for _, point := range currentStrategy.SpeedCurve {
		if effectiveTemp > point.Temp {
			minPoint = point
			continue
		}

		maxPoint = point
		break
	}

	newSpeed := 0
	if minPoint.Temp == maxPoint.Temp && minPoint.Speed == maxPoint.Speed {
		newSpeed = minPoint.Speed
	} else if maxPoint.Temp == minPoint.Temp {
		newSpeed = maxPoint.Speed
	} else {
		slope := float64(maxPoint.Speed-minPoint.Speed) / (maxPoint.Temp - minPoint.Temp)
		newSpeed = int(float64(minPoint.Speed) + (effectiveTemp-minPoint.Temp)*slope)
	}

	f.mu.Lock()
	active := f.active
	f.mu.Unlock()

	if active {
		return f.SetSpeed(newSpeed)
	}

	return nil
}

func (f *FanController) SetSpeed(speed int) error {
	f.mu.Lock()
	f.speed = speed
	err := f.hardware.SetSpeed(speed)
	f.mu.Unlock()

	return err
}

func (f *FanController) OverwriteStrategy(name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	strategy, err := f.config.GetStrategy(name)
	if err != nil {
		return err
	}

	f.overwrittenStrategy = &strategy
	f.timecount = 0

	return nil
}

func (f *FanController) ClearOverwrittenStrategy() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.overwrittenStrategy = nil
	f.timecount = 0
}

func (f *FanController) GetCurrentStrategy() (config.Strategy, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.overwrittenStrategy != nil {
		return *f.overwrittenStrategy, nil
	}

	onAC, err := f.hardware.IsOnAC()
	if err != nil {
		return config.Strategy{}, err
	}

	if onAC {
		return f.config.GetDefaultStrategy()
	}

	return f.config.GetDischargingStrategy()
}

func (f *FanController) Pause() error {
	f.mu.Lock()
	f.active = false
	err := f.hardware.Pause()
	f.mu.Unlock()

	return err
}

func (f *FanController) Resume() error {
	f.mu.Lock()
	f.active = true
	err := f.hardware.Resume()
	f.mu.Unlock()

	return err
}

func (f *FanController) IsActive() bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.active
}

func (f *FanController) GetSpeed() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.speed
}

func (f *FanController) Run(debug bool) error {
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signalCh)

	sleepTimer := time.NewTimer(time.Second)
	defer stopAndDrainTimer(sleepTimer)

	for {
		shutdown, err := f.checkShutdown(signalCh)
		if err != nil {
			return err
		}
		if shutdown {
			return nil
		}

		if !f.IsActive() {
			shutdown, err := f.sleepOrShutdown(5*time.Second, signalCh, sleepTimer)
			if err != nil {
				return err
			}
			if shutdown {
				return nil
			}
			continue
		}

		temp, err := f.GetActualTemperature()
		if err != nil {
			return fmt.Errorf("critical error, exiting for safety reasons: %w", err)
		}

		currentStrategy, err := f.GetCurrentStrategy()
		if err != nil {
			var invalidStrategyErr config.InvalidStrategyError
			if errors.As(err, &invalidStrategyErr) {
				return fmt.Errorf("missing strategy, exiting for safety reasons: %w", err)
			}

			return fmt.Errorf("critical error, exiting for safety reasons: %w", err)
		}

		if currentStrategy.FanSpeedUpdateFrequency <= 0 {
			return fmt.Errorf(
				"critical error, exiting for safety reasons: invalid fanSpeedUpdateFrequency: %d",
				currentStrategy.FanSpeedUpdateFrequency,
			)
		}

		f.mu.Lock()
		shouldAdapt := f.timecount%currentStrategy.FanSpeedUpdateFrequency == 0
		f.mu.Unlock()

		if shouldAdapt {
			if err := f.AdaptSpeed(temp); err != nil {
				var invalidStrategyErr config.InvalidStrategyError
				if errors.As(err, &invalidStrategyErr) {
					return fmt.Errorf("missing strategy, exiting for safety reasons: %w", err)
				}

				return fmt.Errorf("critical error, exiting for safety reasons: %w", err)
			}

			f.mu.Lock()
			f.timecount = 0
			f.mu.Unlock()
		}

		f.pushTemperature(temp)

		if debug {
			fmt.Printf("Strategy='%s' Temp=%.2fC Speed=%d%% Active=%t\n", currentStrategy.Name, temp, f.GetSpeed(), f.IsActive())
		}

		f.mu.Lock()
		f.timecount++
		f.mu.Unlock()

		shutdown, err = f.sleepOrShutdown(1*time.Second, signalCh, sleepTimer)
		if err != nil {
			return err
		}
		if shutdown {
			return nil
		}
	}
}

func (f *FanController) IsDefaultStrategyInUse() bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.overwrittenStrategy == nil
}

func (f *FanController) GetStrategies() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	strategies := f.config.GetStrategies()
	return append([]string(nil), strategies...)
}

func (f *FanController) ReloadConfiguration() error {
	f.mu.Lock()
	err := f.config.Reload()
	overwritten := ""
	if err == nil && f.overwrittenStrategy != nil {
		overwritten = f.overwrittenStrategy.Name
	}
	f.mu.Unlock()

	if err != nil {
		return err
	}

	if overwritten != "" {
		return f.OverwriteStrategy(overwritten)
	}

	return nil
}

func (f *FanController) SetConfiguration(raw []byte) error {
	f.mu.Lock()
	parsed, err := f.config.Parse(raw)
	if err != nil {
		f.mu.Unlock()
		return err
	}

	f.config.Data = parsed

	overwritten := ""
	if f.overwrittenStrategy != nil {
		overwritten = f.overwrittenStrategy.Name
	}

	err = f.config.Save()
	f.mu.Unlock()
	if err != nil {
		return err
	}

	if overwritten != "" {
		return f.OverwriteStrategy(overwritten)
	}

	return nil
}

func (f *FanController) ConfigurationForOutput() map[string]any {
	f.mu.Lock()
	path := f.config.Path
	data := f.config.Data
	f.mu.Unlock()

	return map[string]any{
		"path": path,
		"data": map[string]any{
			"$schema":               data.Schema,
			"defaultStrategy":       data.DefaultStrategy,
			"strategyOnDischarging": data.StrategyOnDischarging,
			"strategies":            data.Strategies,
		},
	}
}

func (f *FanController) StatusSnapshot() (StatusSnapshot, error) {
	currentStrategy, err := f.GetCurrentStrategy()
	if err != nil {
		return StatusSnapshot{}, err
	}

	currentTemp, err := f.GetActualTemperature()
	if err != nil {
		return StatusSnapshot{}, err
	}

	movingAverageTemp, err := f.GetMovingAverageTemperature(currentStrategy.MovingAverageInterval)
	if err != nil {
		return StatusSnapshot{}, err
	}

	effectiveTemp, err := f.GetEffectiveTemperature(currentTemp, currentStrategy.MovingAverageInterval)
	if err != nil {
		return StatusSnapshot{}, err
	}

	f.mu.Lock()
	snapshot := StatusSnapshot{
		Strategy:                 currentStrategy.Name,
		Default:                  f.overwrittenStrategy == nil,
		Speed:                    f.speed,
		Temperature:              currentTemp,
		MovingAverageTemperature: movingAverageTemp,
		EffectiveTemperature:     effectiveTemp,
		Active:                   f.active,
		Configuration: map[string]any{
			"path": f.config.Path,
			"data": map[string]any{
				"$schema":               f.config.Data.Schema,
				"defaultStrategy":       f.config.Data.DefaultStrategy,
				"strategyOnDischarging": f.config.Data.StrategyOnDischarging,
				"strategies":            f.config.Data.Strategies,
			},
		},
	}
	f.mu.Unlock()

	return snapshot, nil
}

func (f *FanController) pushTemperature(temp float64) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if len(f.tempHistory) == 0 {
		f.tempHistory = make([]float64, tempHistoryLimit)
		f.tempHistoryHead = 0
	}

	if len(f.tempHistory) < tempHistoryLimit {
		f.tempHistory = append(f.tempHistory, temp)
		return
	}

	f.tempHistory[f.tempHistoryHead] = temp
	f.tempHistoryHead = (f.tempHistoryHead + 1) % len(f.tempHistory)
}

func (f *FanController) checkShutdown(signalCh <-chan os.Signal) (bool, error) {
	select {
	case <-signalCh:
		if err := f.Pause(); err != nil {
			return true, fmt.Errorf("failed to restore auto fan control on shutdown: %w", err)
		}
		return true, nil
	default:
		return false, nil
	}
}

func (f *FanController) sleepOrShutdown(d time.Duration, signalCh <-chan os.Signal, timer *time.Timer) (bool, error) {
	resetTimer(timer, d)

	select {
	case <-signalCh:
		if err := f.Pause(); err != nil {
			return true, fmt.Errorf("failed to restore auto fan control on shutdown: %w", err)
		}
		return true, nil
	case <-timer.C:
		return false, nil
	}
}

func resetTimer(timer *time.Timer, d time.Duration) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}

	timer.Reset(d)
}

func stopAndDrainTimer(timer *time.Timer) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
}

func round2(value float64) float64 {
	return math.Round(value*100) / 100
}
