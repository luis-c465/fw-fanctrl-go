package dto

import (
	"encoding/json"
	"testing"
)

func TestCommandResultNaturalOutput(t *testing.T) {
	t.Parallel()

	success := NewSuccessCommandResult()
	if got := success.ToOutputFormat(Natural); got != "Success!" {
		t.Fatalf("unexpected success output: %q", got)
	}

	errResult := NewErrorCommandResult("boom")
	if got := errResult.ToOutputFormat(Natural); got != "[Error] > An error occurred: boom" {
		t.Fatalf("unexpected error output: %q", got)
	}
}

func TestStrategyChangeCommandResultOutput(t *testing.T) {
	t.Parallel()

	result := NewStrategyChangeCommandResult("lazy", false)
	if got := result.ToOutputFormat(Natural); got != "Strategy in use: 'lazy'\nDefault: False" {
		t.Fatalf("unexpected natural output: %q", got)
	}

	payload := decodeJSON(t, result.ToOutputFormat(JSON))
	assertString(t, payload, "status", "success")
	assertString(t, payload, "strategy", "lazy")
	assertBool(t, payload, "default", false)
}

func TestPrintStrategyListCommandResultOutput(t *testing.T) {
	t.Parallel()

	result := NewPrintStrategyListCommandResult([]string{"lazy", "medium"})
	if got := result.ToOutputFormat(Natural); got != "Strategy list: \n- lazy\n- medium" {
		t.Fatalf("unexpected natural output: %q", got)
	}

	payload := decodeJSON(t, result.ToOutputFormat(JSON))
	assertString(t, payload, "status", "success")

	strategies, ok := payload["strategies"].([]any)
	if !ok {
		t.Fatalf("strategies has unexpected type: %T", payload["strategies"])
	}

	if len(strategies) != 2 || strategies[0] != "lazy" || strategies[1] != "medium" {
		t.Fatalf("unexpected strategies payload: %v", strategies)
	}
}

func TestSetConfigurationCommandResultOutput(t *testing.T) {
	t.Parallel()

	configuration := map[string]any{
		"path": "/etc/fw-fanctrl/config.json",
		"data": map[string]any{"defaultStrategy": "lazy"},
	}

	result := NewSetConfigurationCommandResult("lazy", configuration, true)
	if got := result.ToOutputFormat(Natural); got == "" {
		t.Fatal("expected non-empty natural output")
	}

	payload := decodeJSON(t, result.ToOutputFormat(JSON))
	assertString(t, payload, "status", "success")
	assertString(t, payload, "strategy", "lazy")
	assertBool(t, payload, "default", true)

	if _, ok := payload["configuration"].(map[string]any); !ok {
		t.Fatalf("configuration has unexpected type: %T", payload["configuration"])
	}
}

func decodeJSON(t *testing.T, raw string) map[string]any {
	t.Helper()

	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("failed to decode json: %v", err)
	}

	return payload
}

func assertString(t *testing.T, payload map[string]any, key string, expected string) {
	t.Helper()

	value, ok := payload[key].(string)
	if !ok {
		t.Fatalf("%s has unexpected type: %T", key, payload[key])
	}

	if value != expected {
		t.Fatalf("unexpected %s: got %q, want %q", key, value, expected)
	}
}

func assertBool(t *testing.T, payload map[string]any, key string, expected bool) {
	t.Helper()

	value, ok := payload[key].(bool)
	if !ok {
		t.Fatalf("%s has unexpected type: %T", key, payload[key])
	}

	if value != expected {
		t.Fatalf("unexpected %s: got %v, want %v", key, value, expected)
	}
}
