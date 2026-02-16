package dto

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	CommandStatusSuccess = "success"
	CommandStatusError   = "error"
)

type CommandResult struct {
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
	Info   string `json:"info,omitempty"`
}

func NewSuccessCommandResult() CommandResult {
	return CommandResult{Status: CommandStatusSuccess}
}

func NewErrorCommandResult(reason string) CommandResult {
	return CommandResult{Status: CommandStatusError, Reason: reason}
}

func (c CommandResult) ToOutputFormat(format OutputFormat) string {
	if format == JSON {
		return c.ToJSON()
	}

	if c.Status == CommandStatusSuccess {
		return "Success!"
	}

	return fmt.Sprintf("[Error] > An error occurred: %s", c.Reason)
}

func (c CommandResult) ToJSON() string {
	return toJSON(c)
}

type StrategyResetCommandResult struct {
	CommandResult
	Strategy string `json:"strategy"`
	Default  bool   `json:"default"`
}

func NewStrategyResetCommandResult(strategy string, defaultInUse bool) StrategyResetCommandResult {
	return StrategyResetCommandResult{
		CommandResult: NewSuccessCommandResult(),
		Strategy:      strategy,
		Default:       defaultInUse,
	}
}

func (r StrategyResetCommandResult) ToOutputFormat(format OutputFormat) string {
	if format == JSON {
		return r.ToJSON()
	}

	return fmt.Sprintf("Strategy reset to default! Strategy in use: '%s'\nDefault: %s", r.Strategy, pythonBool(r.Default))
}

func (r StrategyResetCommandResult) ToJSON() string {
	return toJSON(r)
}

type StrategyChangeCommandResult struct {
	CommandResult
	Strategy string `json:"strategy"`
	Default  bool   `json:"default"`
}

func NewStrategyChangeCommandResult(strategy string, defaultInUse bool) StrategyChangeCommandResult {
	return StrategyChangeCommandResult{
		CommandResult: NewSuccessCommandResult(),
		Strategy:      strategy,
		Default:       defaultInUse,
	}
}

func (r StrategyChangeCommandResult) ToOutputFormat(format OutputFormat) string {
	if format == JSON {
		return r.ToJSON()
	}

	return fmt.Sprintf("Strategy in use: '%s'\nDefault: %s", r.Strategy, pythonBool(r.Default))
}

func (r StrategyChangeCommandResult) ToJSON() string {
	return toJSON(r)
}

type ConfigurationReloadCommandResult struct {
	CommandResult
	Strategy string `json:"strategy"`
	Default  bool   `json:"default"`
}

func NewConfigurationReloadCommandResult(strategy string, defaultInUse bool) ConfigurationReloadCommandResult {
	return ConfigurationReloadCommandResult{
		CommandResult: NewSuccessCommandResult(),
		Strategy:      strategy,
		Default:       defaultInUse,
	}
}

func (r ConfigurationReloadCommandResult) ToOutputFormat(format OutputFormat) string {
	if format == JSON {
		return r.ToJSON()
	}

	return fmt.Sprintf("Reloaded with success! Strategy in use: '%s'\nDefault: %s", r.Strategy, pythonBool(r.Default))
}

func (r ConfigurationReloadCommandResult) ToJSON() string {
	return toJSON(r)
}

type ServicePauseCommandResult struct {
	CommandResult
}

func NewServicePauseCommandResult() ServicePauseCommandResult {
	return ServicePauseCommandResult{CommandResult: NewSuccessCommandResult()}
}

func (r ServicePauseCommandResult) ToOutputFormat(format OutputFormat) string {
	if format == JSON {
		return r.ToJSON()
	}

	return "Service paused! The hardware fan control will take over"
}

func (r ServicePauseCommandResult) ToJSON() string {
	return toJSON(r)
}

type ServiceResumeCommandResult struct {
	CommandResult
	Strategy string `json:"strategy"`
	Default  bool   `json:"default"`
}

func NewServiceResumeCommandResult(strategy string, defaultInUse bool) ServiceResumeCommandResult {
	return ServiceResumeCommandResult{
		CommandResult: NewSuccessCommandResult(),
		Strategy:      strategy,
		Default:       defaultInUse,
	}
}

func (r ServiceResumeCommandResult) ToOutputFormat(format OutputFormat) string {
	if format == JSON {
		return r.ToJSON()
	}

	return fmt.Sprintf("Service resumed!\nStrategy in use: '%s'\nDefault: %s", r.Strategy, pythonBool(r.Default))
}

func (r ServiceResumeCommandResult) ToJSON() string {
	return toJSON(r)
}

type PrintActiveCommandResult struct {
	CommandResult
	Active bool `json:"active"`
}

func NewPrintActiveCommandResult(active bool) PrintActiveCommandResult {
	return PrintActiveCommandResult{CommandResult: NewSuccessCommandResult(), Active: active}
}

func (r PrintActiveCommandResult) ToOutputFormat(format OutputFormat) string {
	if format == JSON {
		return r.ToJSON()
	}

	return fmt.Sprintf("Active: %s", pythonBool(r.Active))
}

func (r PrintActiveCommandResult) ToJSON() string {
	return toJSON(r)
}

type PrintCurrentStrategyCommandResult struct {
	CommandResult
	Strategy string `json:"strategy"`
	Default  bool   `json:"default"`
}

func NewPrintCurrentStrategyCommandResult(strategy string, defaultInUse bool) PrintCurrentStrategyCommandResult {
	return PrintCurrentStrategyCommandResult{
		CommandResult: NewSuccessCommandResult(),
		Strategy:      strategy,
		Default:       defaultInUse,
	}
}

func (r PrintCurrentStrategyCommandResult) ToOutputFormat(format OutputFormat) string {
	if format == JSON {
		return r.ToJSON()
	}

	return fmt.Sprintf("Strategy in use: '%s'\nDefault: %s", r.Strategy, pythonBool(r.Default))
}

func (r PrintCurrentStrategyCommandResult) ToJSON() string {
	return toJSON(r)
}

type PrintStrategyListCommandResult struct {
	CommandResult
	Strategies []string `json:"strategies"`
}

func NewPrintStrategyListCommandResult(strategies []string) PrintStrategyListCommandResult {
	return PrintStrategyListCommandResult{CommandResult: NewSuccessCommandResult(), Strategies: strategies}
}

func (r PrintStrategyListCommandResult) ToOutputFormat(format OutputFormat) string {
	if format == JSON {
		return r.ToJSON()
	}

	return fmt.Sprintf("Strategy list: \n- %s", strings.Join(r.Strategies, "\n- "))
}

func (r PrintStrategyListCommandResult) ToJSON() string {
	return toJSON(r)
}

type PrintFanSpeedCommandResult struct {
	CommandResult
	Speed string `json:"speed"`
}

func NewPrintFanSpeedCommandResult(speed string) PrintFanSpeedCommandResult {
	return PrintFanSpeedCommandResult{CommandResult: NewSuccessCommandResult(), Speed: speed}
}

func (r PrintFanSpeedCommandResult) ToOutputFormat(format OutputFormat) string {
	if format == JSON {
		return r.ToJSON()
	}

	return fmt.Sprintf("Current fan speed: '%s%%'", r.Speed)
}

func (r PrintFanSpeedCommandResult) ToJSON() string {
	return toJSON(r)
}

type SetConfigurationCommandResult struct {
	CommandResult
	Strategy      string `json:"strategy"`
	Configuration any    `json:"configuration"`
	Default       bool   `json:"default"`
}

func NewSetConfigurationCommandResult(strategy string, configuration any, defaultInUse bool) SetConfigurationCommandResult {
	return SetConfigurationCommandResult{
		CommandResult: NewSuccessCommandResult(),
		Strategy:      strategy,
		Configuration: configuration,
		Default:       defaultInUse,
	}
}

func (r SetConfigurationCommandResult) ToOutputFormat(format OutputFormat) string {
	if format == JSON {
		return r.ToJSON()
	}

	return fmt.Sprintf(
		"Configuration updated with success: %s.\nStrategy in use: %s\nDefault: %s",
		toJSON(r.Configuration),
		r.Strategy,
		pythonBool(r.Default),
	)
}

func (r SetConfigurationCommandResult) ToJSON() string {
	return toJSON(r)
}

func pythonBool(value bool) string {
	if value {
		return "True"
	}

	return "False"
}

func toJSON(v any) string {
	payload, err := json.Marshal(v)
	if err != nil {
		fallback, _ := json.Marshal(CommandResult{Status: CommandStatusError, Reason: fmt.Sprintf("failed to serialize result: %v", err)})
		return string(fallback)
	}

	return string(payload)
}
