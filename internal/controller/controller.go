package controller

import (
	"errors"
	"fmt"
	"math"
	"os"
	"os/signal"
	"strconv"
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
	Zones                    []ZoneSnapshot
	Active                   bool
	Configuration            map[string]any
}

type ZoneSnapshot struct {
	Name                     string
	Sensors                  []string
	Temperature              float64
	MovingAverageTemperature float64
	EffectiveTemperature     float64
	ComputedSpeed            int
}

type zoneState struct {
	name        string
	sensors     []string
	curve       []config.SpeedCurvePoint
	maInterval  int
	history     []float64
	historyHead int
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
	zones               []zoneState
	zoneStrategyKey     string
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

func (f *FanController) GetTemperatureReadings() ([]hardware.SensorReading, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.hardware.GetTemperatures()
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

func (f *FanController) AdaptSpeed(currentTemp float64, readings []hardware.SensorReading) error {
	currentStrategy, err := f.GetCurrentStrategy()
	if err != nil {
		return err
	}

	newSpeed := 0
	if currentStrategy.IsMultiSensor() {
		newSpeed, err = f.adaptSpeedForSensorCurves(currentStrategy, readings)
		if err != nil {
			return err
		}
	} else {
		if len(currentStrategy.SpeedCurve) == 0 {
			return errors.New("current strategy has empty speed curve")
		}

		effectiveTemp, err := f.GetEffectiveTemperature(currentTemp, currentStrategy.MovingAverageInterval)
		if err != nil {
			return err
		}

		newSpeed = interpolateSpeed(effectiveTemp, currentStrategy.SpeedCurve)
	}

	f.mu.Lock()
	active := f.active
	f.mu.Unlock()

	if active {
		return f.SetSpeed(newSpeed)
	}

	return nil
}

func (f *FanController) adaptSpeedForSensorCurves(strategy config.Strategy, readings []hardware.SensorReading) (int, error) {
	f.ensureZoneState(strategy)

	f.mu.Lock()
	zones := make([]zoneState, len(f.zones))
	copy(zones, f.zones)
	f.mu.Unlock()

	if len(zones) == 0 {
		return 0, errors.New("current strategy has empty sensor curves")
	}

	readingByName := make(map[string]hardware.SensorReading, len(readings))
	readingByIndex := make(map[int]hardware.SensorReading, len(readings))
	for _, reading := range readings {
		readingByName[reading.Name] = reading
		readingByIndex[reading.Index] = reading
	}

	maxSpeed := 0
	for i := range zones {
		zoneTemp := 50.0
		found := false

		for _, sensorRef := range zones[i].sensors {
			if reading, ok := readingByName[sensorRef]; ok {
				if !found || reading.TempC > zoneTemp {
					zoneTemp = reading.TempC
					found = true
				}
				continue
			}

			idx, err := strconv.Atoi(sensorRef)
			if err != nil {
				continue
			}
			if reading, ok := readingByIndex[idx]; ok {
				if !found || reading.TempC > zoneTemp {
					zoneTemp = reading.TempC
					found = true
				}
			}
		}

		effectiveTemp := f.pushZoneTemperatureAndGetEffectiveTemp(i, zoneTemp, zones[i].maInterval)
		speed := interpolateSpeed(effectiveTemp, zones[i].curve)
		if speed > maxSpeed {
			maxSpeed = speed
		}
	}

	return maxSpeed, nil
}

func interpolateSpeed(effectiveTemp float64, curve []config.SpeedCurvePoint) int {
	minPoint := curve[0]
	maxPoint := curve[len(curve)-1]

	for _, point := range curve {
		if effectiveTemp > point.Temp {
			minPoint = point
			continue
		}

		maxPoint = point
		break
	}

	if minPoint.Temp == maxPoint.Temp && minPoint.Speed == maxPoint.Speed {
		return minPoint.Speed
	}
	if maxPoint.Temp == minPoint.Temp {
		return maxPoint.Speed
	}

	slope := float64(maxPoint.Speed-minPoint.Speed) / (maxPoint.Temp - minPoint.Temp)
	return int(float64(minPoint.Speed) + (effectiveTemp-minPoint.Temp)*slope)
}

func (f *FanController) ensureZoneState(strategy config.Strategy) {
	strategyKey := strategy.Name
	if !strategy.IsMultiSensor() {
		f.mu.Lock()
		f.zones = nil
		f.zoneStrategyKey = strategyKey
		f.mu.Unlock()
		return
	}

	f.mu.Lock()
	if f.zoneStrategyKey == strategyKey && len(f.zones) == len(strategy.SensorCurves) {
		f.mu.Unlock()
		return
	}

	zones := make([]zoneState, 0, len(strategy.SensorCurves))
	for _, sensorCurve := range strategy.SensorCurves {
		zones = append(zones, zoneState{
			name:        sensorCurve.Name,
			sensors:     append([]string(nil), sensorCurve.Sensors...),
			curve:       append([]config.SpeedCurvePoint(nil), sensorCurve.SpeedCurve...),
			maInterval:  sensorCurve.MovingAverageInterval,
			history:     make([]float64, tempHistoryLimit),
			historyHead: 0,
		})
	}

	f.zones = zones
	f.zoneStrategyKey = strategyKey
	f.mu.Unlock()
}

func (f *FanController) pushZoneTemperatureAndGetEffectiveTemp(zoneIndex int, currentTemp float64, timeInterval int) float64 {
	f.mu.Lock()
	if zoneIndex < 0 || zoneIndex >= len(f.zones) {
		f.mu.Unlock()
		return currentTemp
	}

	zone := &f.zones[zoneIndex]
	if len(zone.history) == 0 {
		zone.history = make([]float64, tempHistoryLimit)
		zone.historyHead = 0
	}

	zone.history[zone.historyHead] = currentTemp
	zone.historyHead = (zone.historyHead + 1) % len(zone.history)

	historyLen := len(zone.history)
	targetNonZero := historyLen
	if timeInterval > 0 {
		targetNonZero = timeInterval
	}

	total := 0.0
	nonZeroCount := 0
	for i := 0; i < historyLen; i++ {
		idx := zone.historyHead - 1 - i
		if idx < 0 {
			idx += historyLen
		}

		temp := zone.history[idx]
		if temp > 0 {
			total += temp
			nonZeroCount++
			if nonZeroCount == targetNonZero {
				break
			}
		}
	}
	f.mu.Unlock()

	if nonZeroCount == 0 {
		return round2(currentTemp)
	}

	movingAverage := round2(total / float64(nonZeroCount))
	return round2(math.Min(movingAverage, currentTemp))
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
	f.zoneStrategyKey = ""
	f.zones = nil

	return nil
}

func (f *FanController) ClearOverwrittenStrategy() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.overwrittenStrategy = nil
	f.timecount = 0
	f.zoneStrategyKey = ""
	f.zones = nil
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

		readings, err := f.GetTemperatureReadings()
		if err != nil {
			return fmt.Errorf("critical error, exiting for safety reasons: %w", err)
		}

		temp := highestFromReadings(readings)

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
			if err := f.AdaptSpeed(temp, readings); err != nil {
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
	f.zoneStrategyKey = ""
	f.zones = nil
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
	f.zoneStrategyKey = ""
	f.zones = nil
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

	readings, err := f.GetTemperatureReadings()
	if err != nil {
		return StatusSnapshot{}, err
	}
	currentTemp := highestFromReadings(readings)

	movingAverageTemp, err := f.GetMovingAverageTemperature(currentStrategy.MovingAverageInterval)
	if err != nil {
		return StatusSnapshot{}, err
	}

	effectiveTemp, err := f.GetEffectiveTemperature(currentTemp, currentStrategy.MovingAverageInterval)
	if err != nil {
		return StatusSnapshot{}, err
	}

	zones := []ZoneSnapshot{}
	if currentStrategy.IsMultiSensor() {
		f.ensureZoneState(currentStrategy)

		readingByName := make(map[string]hardware.SensorReading, len(readings))
		readingByIndex := make(map[int]hardware.SensorReading, len(readings))
		for _, reading := range readings {
			readingByName[reading.Name] = reading
			readingByIndex[reading.Index] = reading
		}

		f.mu.Lock()
		zonesState := make([]zoneState, len(f.zones))
		copy(zonesState, f.zones)
		f.mu.Unlock()

		zones = make([]ZoneSnapshot, 0, len(zonesState))
		for _, zone := range zonesState {
			zoneTemp := 50.0
			found := false
			for _, sensorRef := range zone.sensors {
				if reading, ok := readingByName[sensorRef]; ok {
					if !found || reading.TempC > zoneTemp {
						zoneTemp = reading.TempC
						found = true
					}
					continue
				}

				idx, parseErr := strconv.Atoi(sensorRef)
				if parseErr != nil {
					continue
				}
				if reading, ok := readingByIndex[idx]; ok {
					if !found || reading.TempC > zoneTemp {
						zoneTemp = reading.TempC
						found = true
					}
				}
			}

			movingAverage := zoneTemp
			if len(zone.history) > 0 {
				total := 0.0
				nonZero := 0
				target := len(zone.history)
				if zone.maInterval > 0 {
					target = zone.maInterval
				}
				for i := 0; i < len(zone.history); i++ {
					idx := zone.historyHead - 1 - i
					if idx < 0 {
						idx += len(zone.history)
					}
					temp := zone.history[idx]
					if temp > 0 {
						total += temp
						nonZero++
						if nonZero == target {
							break
						}
					}
				}
				if nonZero > 0 {
					movingAverage = round2(total / float64(nonZero))
				}
			}

			effective := round2(math.Min(movingAverage, zoneTemp))
			zoneSpeed := interpolateSpeed(effective, zone.curve)
			zones = append(zones, ZoneSnapshot{
				Name:                     zone.name,
				Sensors:                  append([]string(nil), zone.sensors...),
				Temperature:              round2(zoneTemp),
				MovingAverageTemperature: round2(movingAverage),
				EffectiveTemperature:     effective,
				ComputedSpeed:            zoneSpeed,
			})
		}
	}

	f.mu.Lock()
	snapshot := StatusSnapshot{
		Strategy:                 currentStrategy.Name,
		Default:                  f.overwrittenStrategy == nil,
		Speed:                    f.speed,
		Temperature:              currentTemp,
		MovingAverageTemperature: movingAverageTemp,
		EffectiveTemperature:     effectiveTemp,
		Zones:                    zones,
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

func highestFromReadings(readings []hardware.SensorReading) float64 {
	if len(readings) == 0 {
		return 50.0
	}

	maxTemp := readings[0].TempC
	for _, reading := range readings[1:] {
		if reading.TempC > maxTemp {
			maxTemp = reading.TempC
		}
	}

	return round2(maxTemp)
}
