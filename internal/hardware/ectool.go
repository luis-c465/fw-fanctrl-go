package hardware

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	ectoolCommandTimeout = 5 * time.Second
	acStatusCacheTTL     = 10 * time.Second
)

var (
	batterySensorRegexp = regexp.MustCompile(`\d+ Battery`)
	sensorIDRegexp      = regexp.MustCompile(`(?m)^\d+`)
	temperatureRegexp   = regexp.MustCompile(`\(=\s*(\d+)\s*C\)`)
	namedTempRegexp     = regexp.MustCompile(`^\s*(\S+)\s+\d+\s+K\s+\(=\s*(\d+)\s*C\)`)
	numericTempRegexp   = regexp.MustCompile(`^\s*(\d+):\s+\d+\s+K\s+\(=\s*(\d+)\s*C\)`)
	acPresentRegexp     = regexp.MustCompile(`Flags.*(AC_PRESENT)`)
)

type EctoolHardwareController struct {
	noBatterySensorMode bool
	nonBatterySensors   []string
	lastACCheck         time.Time
	lastACResult        bool
}

func NewEctoolHardwareController(noBatterySensorMode bool) (*EctoolHardwareController, error) {
	controller := &EctoolHardwareController{noBatterySensorMode: noBatterySensorMode}

	if noBatterySensorMode {
		if err := controller.populateNonBatterySensors(); err != nil {
			return nil, err
		}
	}

	return controller, nil
}

func (c *EctoolHardwareController) populateNonBatterySensors() error {
	output, err := runEctoolCommand(false, "tempsinfo", "all")
	if err != nil {
		return err
	}

	c.nonBatterySensors = parseNonBatterySensors(output)
	return nil
}

func (c *EctoolHardwareController) GetTemperature() (float64, error) {
	readings, err := c.GetTemperatures()
	if err != nil {
		return 0, err
	}

	if len(readings) == 0 {
		return 50.0, nil
	}

	maxTemp := readings[0].TempC
	for _, reading := range readings[1:] {
		if reading.TempC > maxTemp {
			maxTemp = reading.TempC
		}
	}

	return math.Round(maxTemp*100) / 100, nil
}

func (c *EctoolHardwareController) GetTemperatures() ([]SensorReading, error) {
	if c.noBatterySensorMode {
		readings := make([]SensorReading, 0, len(c.nonBatterySensors))

		for _, sensorID := range c.nonBatterySensors {
			sensorOutput, err := runEctoolCommand(false, "temps", sensorID)
			if err != nil {
				return nil, err
			}

			parsed := parseSensorReadings(sensorOutput)
			if len(parsed) == 0 {
				continue
			}

			readings = append(readings, parsed...)
		}

		return readings, nil
	}

	rawOutput, err := runEctoolCommand(false, "temps", "all")
	if err != nil {
		return nil, err
	}

	return parseSensorReadings(rawOutput), nil
}

func (c *EctoolHardwareController) SetSpeed(speed int) error {
	_, err := runEctoolCommand(false, "fanduty", strconv.Itoa(speed))
	return err
}

func (c *EctoolHardwareController) Pause() error {
	_, err := runEctoolCommand(false, "autofanctrl")
	return err
}

func (c *EctoolHardwareController) Resume() error {
	return nil
}

func (c *EctoolHardwareController) IsOnAC() (bool, error) {
	if !c.lastACCheck.IsZero() && time.Since(c.lastACCheck) < acStatusCacheTTL {
		return c.lastACResult, nil
	}

	output, err := runEctoolCommand(true, "battery")
	if err != nil {
		return false, err
	}

	c.lastACResult = parseACPresent(output)
	c.lastACCheck = time.Now()

	return c.lastACResult, nil
}

func parseTemperatures(output string) []int {
	matches := temperatureRegexp.FindAllStringSubmatch(output, -1)
	temps := make([]int, 0, len(matches))

	for _, match := range matches {
		temp, err := strconv.Atoi(match[1])
		if err != nil {
			continue
		}
		if temp > 0 {
			temps = append(temps, temp)
		}
	}

	sort.Sort(sort.Reverse(sort.IntSlice(temps)))
	return temps
}

func parseSensorReadings(output string) []SensorReading {
	lines := strings.Split(output, "\n")
	readings := make([]SensorReading, 0, len(lines))
	sensorPos := 0

	for idx, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if matches := numericTempRegexp.FindStringSubmatch(line); len(matches) == 3 {
			temp, err := strconv.Atoi(matches[2])
			if err != nil || temp <= 0 {
				continue
			}

			name := matches[1]
			parsedIndex, err := strconv.Atoi(matches[1])
			if err != nil {
				parsedIndex = idx
			}

			readings = append(readings, SensorReading{
				Name:    name,
				Index:   parsedIndex,
				TempC:   math.Round(float64(temp)*100) / 100,
				Present: true,
			})
			sensorPos++
			continue
		}

		if matches := namedTempRegexp.FindStringSubmatch(line); len(matches) == 3 {
			temp, err := strconv.Atoi(matches[2])
			if err != nil || temp <= 0 {
				continue
			}

			readings = append(readings, SensorReading{
				Name:    matches[1],
				Index:   sensorPos,
				TempC:   math.Round(float64(temp)*100) / 100,
				Present: true,
			})
			sensorPos++
		}
	}

	return readings
}

func highestTemperatureOrFallback(output string) float64 {
	temps := parseTemperatures(output)
	if len(temps) == 0 {
		return 50.0
	}

	maxTemp := temps[0]
	for _, temp := range temps[1:] {
		if temp > maxTemp {
			maxTemp = temp
		}
	}

	return math.Round(float64(maxTemp)*100) / 100
}

func parseACPresent(output string) bool {
	return acPresentRegexp.MatchString(output)
}

func parseNonBatterySensors(output string) []string {
	batteryMatches := batterySensorRegexp.FindAllString(output, -1)
	batterySensors := make(map[string]struct{}, len(batteryMatches))

	for _, match := range batteryMatches {
		parts := strings.SplitN(match, " ", 2)
		if len(parts) == 0 {
			continue
		}
		batterySensors[parts[0]] = struct{}{}
	}

	allSensors := sensorIDRegexp.FindAllString(output, -1)
	nonBatterySensors := make([]string, 0, len(allSensors))

	for _, sensor := range allSensors {
		if _, isBattery := batterySensors[sensor]; isBattery {
			continue
		}
		nonBatterySensors = append(nonBatterySensors, sensor)
	}

	return nonBatterySensors
}

func runEctoolCommand(discardStderr bool, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), ectoolCommandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ectool", args...)
	if discardStderr {
		cmd.Stderr = io.Discard
	}

	output, err := cmd.Output()
	if err != nil {
		return "", buildEctoolError(err, args)
	}

	return string(output), nil
}

func buildEctoolError(err error, args []string) error {
	commandText := fmt.Sprintf("ectool %s", strings.Join(args, " "))

	if errors.Is(err, exec.ErrNotFound) {
		return fmt.Errorf("failed to execute %q: ectool not found in PATH", commandText)
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("failed to execute %q: command timed out after %s", commandText, ectoolCommandTimeout)
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		stderr := strings.TrimSpace(string(exitErr.Stderr))
		if stderr == "" {
			return fmt.Errorf("failed to execute %q: %w", commandText, err)
		}
		return fmt.Errorf("failed to execute %q: %s", commandText, stderr)
	}

	return fmt.Errorf("failed to execute %q: %w", commandText, err)
}
