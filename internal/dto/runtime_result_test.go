package dto

import "testing"

func TestStatusRuntimeResultNaturalOutput(t *testing.T) {
	t.Parallel()

	result := NewStatusRuntimeResult(
		"lazy",
		true,
		25,
		54,
		52.5,
		52.5,
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
		false,
		map[string]any{"path": "/tmp/config.json", "data": map[string]any{"defaultStrategy": "lazy"}},
	)

	payload := decodeJSON(t, result.ToOutputFormat(JSON))
	assertString(t, payload, "status", "success")
	assertString(t, payload, "strategy", "lazy")
	assertBool(t, payload, "default", false)
	assertBool(t, payload, "active", false)
}
