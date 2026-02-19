package dto

import (
	"strings"
	"testing"
)

func TestRuntimeResultNaturalOutput(t *testing.T) {
	t.Parallel()

	success := NewSuccessRuntimeResult()
	if got := success.ToOutputFormat(Natural); got != "Success!" {
		t.Fatalf("unexpected success output: %q", got)
	}

	errResult := NewErrorRuntimeResult("boom")
	if got := errResult.ToOutputFormat(Natural); got != "[Error] > An error occurred: boom" {
		t.Fatalf("unexpected error output: %q", got)
	}
}

func TestStatusRuntimeResultNaturalOutput(t *testing.T) {
	t.Parallel()

	result := NewStatusRuntimeResult(
		"lazy",
		true,
		25,
		54,
		52.5,
		52.5,
		nil,
		true,
		map[string]any{
			"path": "/etc/fw-fanctrl/config.json",
			"data": map[string]any{
				"defaultStrategy":       "lazy",
				"strategyOnDischarging": "",
			},
		},
	)

	got := result.ToOutputFormat(Natural)
	want := "Strategy: 'lazy'\n" +
		"Default: True\n" +
		"Speed: 25%\n" +
		"Temp: 54.0°C\n" +
		"MovingAverageTemp: 52.5°C\n" +
		"EffectiveTemp: 52.5°C\n" +
		"Active: True\n" +
		"DefaultStrategy: 'lazy'\n" +
		"DischargingStrategy: ''\n"

	if got != want {
		t.Fatalf("unexpected natural output:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestStatusRuntimeResultJSONOutput(t *testing.T) {
	t.Parallel()

	result := NewStatusRuntimeResult(
		"lazy",
		false,
		30,
		60,
		58,
		58,
		nil,
		false,
		map[string]any{"path": "/tmp/config.json", "data": map[string]any{"defaultStrategy": "lazy"}},
	)

	payload := decodeJSON(t, result.ToOutputFormat(JSON))
	assertString(t, payload, "status", "success")
	assertString(t, payload, "strategy", "lazy")
	assertBool(t, payload, "default", false)
	assertBool(t, payload, "active", false)
}

func TestStatusRuntimeResultNaturalOutputHandlesMissingConfigurationFields(t *testing.T) {
	t.Parallel()

	result := NewStatusRuntimeResult("lazy", true, 25, 54, 52, 52, nil, true, map[string]any{})
	got := result.ToOutputFormat(Natural)

	if got == "" {
		t.Fatal("expected non-empty natural output")
	}

	if expected := "DefaultStrategy: ''"; !containsLine(got, expected) {
		t.Fatalf("expected output to contain %q, got %q", expected, got)
	}

	if expected := "DischargingStrategy: ''"; !containsLine(got, expected) {
		t.Fatalf("expected output to contain %q, got %q", expected, got)
	}
}

func TestPythonFloatFormatting(t *testing.T) {
	t.Parallel()

	if got := pythonFloat(54); got != "54.0" {
		t.Fatalf("unexpected integer float format: %q", got)
	}

	if got := pythonFloat(52.5); got != "52.5" {
		t.Fatalf("unexpected decimal float format: %q", got)
	}
}

func TestStatusRuntimeResultNaturalOutputWithZones(t *testing.T) {
	t.Parallel()

	result := NewStatusRuntimeResult(
		"multi",
		true,
		35,
		53,
		50,
		50,
		[]ZoneResult{
			{
				Name:                     "cpu",
				Sensors:                  []string{"cpu@4c"},
				Temperature:              53,
				MovingAverageTemperature: 50,
				EffectiveTemperature:     50,
				ComputedSpeed:            35,
			},
		},
		true,
		map[string]any{},
	)

	got := result.ToOutputFormat(Natural)
	if !strings.Contains(got, "Zones:") {
		t.Fatalf("expected zones block in output, got %q", got)
	}
	if !strings.Contains(got, "cpu") {
		t.Fatalf("expected cpu zone in output, got %q", got)
	}
}

func containsLine(output string, expected string) bool {
	return strings.Contains(output, expected)
}
