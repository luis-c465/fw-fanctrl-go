package dto

import (
	"fmt"
	"strconv"
	"strings"
)

type RuntimeResult struct {
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
	Info   string `json:"info,omitempty"`
}

func NewSuccessRuntimeResult() RuntimeResult {
	return RuntimeResult{Status: CommandStatusSuccess}
}

func NewErrorRuntimeResult(reason string) RuntimeResult {
	return RuntimeResult{Status: CommandStatusError, Reason: reason}
}

func (r RuntimeResult) ToOutputFormat(format OutputFormat) string {
	if format == JSON {
		return r.ToJSON()
	}

	if r.Status == CommandStatusSuccess {
		return "Success!"
	}

	return fmt.Sprintf("[Error] > An error occurred: %s", r.Reason)
}

func (r RuntimeResult) ToJSON() string {
	return toJSON(r)
}

type StatusRuntimeResult struct {
	RuntimeResult
	Strategy                 string         `json:"strategy"`
	Default                  bool           `json:"default"`
	Speed                    int            `json:"speed"`
	Temperature              float64        `json:"temperature"`
	MovingAverageTemperature float64        `json:"movingAverageTemperature"`
	EffectiveTemperature     float64        `json:"effectiveTemperature"`
	Zones                    []ZoneResult   `json:"zones,omitempty"`
	Active                   bool           `json:"active"`
	Configuration            map[string]any `json:"configuration"`
}

type ZoneResult struct {
	Name                     string   `json:"name"`
	Sensors                  []string `json:"sensors"`
	Temperature              float64  `json:"temperature"`
	MovingAverageTemperature float64  `json:"movingAverageTemperature"`
	EffectiveTemperature     float64  `json:"effectiveTemperature"`
	ComputedSpeed            int      `json:"computedSpeed"`
}

func NewStatusRuntimeResult(
	strategy string,
	defaultInUse bool,
	speed int,
	temperature float64,
	movingAverageTemperature float64,
	effectiveTemperature float64,
	zones []ZoneResult,
	active bool,
	configuration map[string]any,
) StatusRuntimeResult {
	return StatusRuntimeResult{
		RuntimeResult:            NewSuccessRuntimeResult(),
		Strategy:                 strategy,
		Default:                  defaultInUse,
		Speed:                    speed,
		Temperature:              temperature,
		MovingAverageTemperature: movingAverageTemperature,
		EffectiveTemperature:     effectiveTemperature,
		Zones:                    zones,
		Active:                   active,
		Configuration:            configuration,
	}
}

func (r StatusRuntimeResult) ToOutputFormat(format OutputFormat) string {
	if format == JSON {
		return r.ToJSON()
	}

	defaultStrategy := ""
	dischargingStrategy := ""
	if data, ok := r.Configuration["data"].(map[string]any); ok {
		if value, exists := data["defaultStrategy"].(string); exists {
			defaultStrategy = value
		}
		if value, exists := data["strategyOnDischarging"].(string); exists {
			dischargingStrategy = value
		}
	}

	base := fmt.Sprintf(
		"Strategy: '%s'\nDefault: %s\nSpeed: %d%%\nTemp: %s°C\nMovingAverageTemp: %s°C\nEffectiveTemp: %s°C\nActive: %s\nDefaultStrategy: '%s'\nDischargingStrategy: '%s'\n",
		r.Strategy,
		pythonBool(r.Default),
		r.Speed,
		pythonFloat(r.Temperature),
		pythonFloat(r.MovingAverageTemperature),
		pythonFloat(r.EffectiveTemperature),
		pythonBool(r.Active),
		defaultStrategy,
		dischargingStrategy,
	)

	if len(r.Zones) == 0 {
		return base
	}

	b := strings.Builder{}
	b.WriteString(base)
	b.WriteString("Zones:\n")
	for _, zone := range r.Zones {
		b.WriteString(fmt.Sprintf(
			"- %s: Sensors=%s Temp=%s°C MovingAverageTemp=%s°C EffectiveTemp=%s°C ComputedSpeed=%d%%\n",
			zone.Name,
			strings.Join(zone.Sensors, ","),
			pythonFloat(zone.Temperature),
			pythonFloat(zone.MovingAverageTemperature),
			pythonFloat(zone.EffectiveTemperature),
			zone.ComputedSpeed,
		))
	}

	return b.String()
}

func (r StatusRuntimeResult) ToJSON() string {
	return toJSON(r)
}

func pythonFloat(value float64) string {
	raw := strconv.FormatFloat(value, 'f', -1, 64)
	if strings.ContainsAny(raw, ".eE") {
		return raw
	}

	return raw + ".0"
}
