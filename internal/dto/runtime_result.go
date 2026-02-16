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
	Active                   bool           `json:"active"`
	Configuration            map[string]any `json:"configuration"`
}

func NewStatusRuntimeResult(
	strategy string,
	defaultInUse bool,
	speed int,
	temperature float64,
	movingAverageTemperature float64,
	effectiveTemperature float64,
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

	return fmt.Sprintf(
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
