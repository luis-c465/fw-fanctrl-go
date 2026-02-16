package hardware

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseTemperatures(t *testing.T) {
	t.Parallel()

	output := `
0: 300 K (= 27 C)
1: 0 K (= 0 C)
2: 335 K (= 62 C)
3: 328 K (= 55 C)
`

	got := parseTemperatures(output)
	want := []int{62, 55, 27}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseTemperatures() mismatch: got %v, want %v", got, want)
	}
}

func TestHighestTemperatureOrFallbackReturnsFallbackWhenEmpty(t *testing.T) {
	t.Parallel()

	got := highestTemperatureOrFallback("no valid temps here")
	if got != 50.0 {
		t.Fatalf("expected safety fallback 50.0, got %.2f", got)
	}
}

func TestHighestTemperatureOrFallbackReturnsHighestTemperature(t *testing.T) {
	t.Parallel()

	output := "0: 300 K (= 27 C)\n2: 335 K (= 62 C)\n"
	if got := highestTemperatureOrFallback(output); got != 62.0 {
		t.Fatalf("expected highest temperature 62.0, got %.2f", got)
	}
}

func TestParseACPresent(t *testing.T) {
	t.Parallel()

	withAC := `Flags: BATT_PRESENT AC_PRESENT CHARGING`
	withoutAC := `Flags: BATT_PRESENT DISCHARGING`

	if !parseACPresent(withAC) {
		t.Fatal("expected AC_PRESENT to be detected")
	}

	if parseACPresent(withoutAC) {
		t.Fatal("expected AC_PRESENT to be absent")
	}
}

func TestParseNonBatterySensors(t *testing.T) {
	t.Parallel()

	output := `
0 CPU
1 Battery
2 DDR
3 Battery
4 Ambient
`

	got := parseNonBatterySensors(output)
	want := []string{"0", "2", "4"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseNonBatterySensors() mismatch: got %v, want %v", got, want)
	}
}

func TestBuildEctoolErrorNotFound(t *testing.T) {
	t.Parallel()

	err := buildEctoolError(exec.ErrNotFound, []string{"temps", "all"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "ectool not found in PATH") {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
}

func TestBuildEctoolErrorTimeout(t *testing.T) {
	t.Parallel()

	err := buildEctoolError(context.DeadlineExceeded, []string{"battery"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "command timed out") {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
}

func TestBuildEctoolErrorExitErrorIncludesStderr(t *testing.T) {
	t.Parallel()

	cmd := exec.Command("sh", "-c", "echo denied 1>&2; exit 1")
	_, cmdErr := cmd.Output()
	if cmdErr == nil {
		t.Fatal("expected command to fail")
	}

	err := buildEctoolError(cmdErr, []string{"battery"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "denied") {
		t.Fatalf("expected stderr in message, got %q", err.Error())
	}
}

func TestBuildEctoolErrorUnknownWrapped(t *testing.T) {
	t.Parallel()

	root := errors.New("boom")
	err := buildEctoolError(root, []string{"temps", "all"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected wrapped message, got %q", err.Error())
	}
}

func TestRunEctoolCommandAndControllerIntegration(t *testing.T) {
	scriptDir := t.TempDir()
	scriptPath := filepath.Join(scriptDir, "ectool")

	script := "#!/bin/sh\n" +
		"cmd=\"$1\"\n" +
		"shift\n" +
		"case \"$cmd\" in\n" +
		"  temps)\n" +
		"    if [ \"$1\" = \"all\" ]; then\n" +
		"      printf \"0: 300 K (= 27 C)\\n2: 335 K (= 62 C)\\n\"\n" +
		"    elif [ \"$1\" = \"0\" ]; then\n" +
		"      printf \"0: 300 K (= 27 C)\\n\"\n" +
		"    elif [ \"$1\" = \"2\" ]; then\n" +
		"      printf \"2: 335 K (= 62 C)\\n\"\n" +
		"    fi\n" +
		"    ;;\n" +
		"  tempsinfo)\n" +
		"    printf \"0 CPU\\n1 Battery\\n2 DDR\\n\"\n" +
		"    ;;\n" +
		"  fanduty)\n" +
		"    printf \"ok\\n\"\n" +
		"    ;;\n" +
		"  autofanctrl)\n" +
		"    printf \"auto\\n\"\n" +
		"    ;;\n" +
		"  battery)\n" +
		"    printf \"Flags: BATT_PRESENT AC_PRESENT CHARGING\\n\"\n" +
		"    ;;\n" +
		"  fail)\n" +
		"    printf \"denied\\n\" 1>&2\n" +
		"    exit 1\n" +
		"    ;;\n" +
		"esac\n"

	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to create fake ectool: %v", err)
	}

	t.Setenv("PATH", scriptDir)

	output, err := runEctoolCommand(false, "temps", "all")
	if err != nil {
		t.Fatalf("runEctoolCommand() returned error: %v", err)
	}

	if !strings.Contains(output, "62 C") {
		t.Fatalf("unexpected output: %q", output)
	}

	controller, err := NewEctoolHardwareController(true)
	if err != nil {
		t.Fatalf("NewEctoolHardwareController() returned error: %v", err)
	}

	if !reflect.DeepEqual(controller.nonBatterySensors, []string{"0", "2"}) {
		t.Fatalf("unexpected non-battery sensors: %v", controller.nonBatterySensors)
	}

	temp, err := controller.GetTemperature()
	if err != nil {
		t.Fatalf("GetTemperature() returned error: %v", err)
	}

	if temp != 62.0 {
		t.Fatalf("expected temperature 62.0, got %.2f", temp)
	}

	if err := controller.SetSpeed(35); err != nil {
		t.Fatalf("SetSpeed() returned error: %v", err)
	}

	if err := controller.Pause(); err != nil {
		t.Fatalf("Pause() returned error: %v", err)
	}

	onAC, err := controller.IsOnAC()
	if err != nil {
		t.Fatalf("IsOnAC() returned error: %v", err)
	}

	if !onAC {
		t.Fatal("expected IsOnAC() to return true")
	}
}

func TestRunEctoolCommandNotFound(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	_, err := runEctoolCommand(false, "temps", "all")
	if err == nil {
		t.Fatal("expected error when ectool is missing")
	}

	if !strings.Contains(err.Error(), "ectool not found in PATH") {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
}

func TestRunEctoolCommandReturnsStderrOnFailure(t *testing.T) {
	scriptDir := t.TempDir()
	scriptPath := filepath.Join(scriptDir, "ectool")

	script := "#!/bin/sh\n" +
		"printf \"denied\\n\" 1>&2\n" +
		"exit 1\n"

	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to create fake ectool: %v", err)
	}

	t.Setenv("PATH", scriptDir)

	_, err := runEctoolCommand(false, "battery")
	if err == nil {
		t.Fatal("expected ectool command failure")
	}

	if !strings.Contains(err.Error(), "denied") {
		t.Fatalf("expected stderr content in error, got %q", err.Error())
	}
}
