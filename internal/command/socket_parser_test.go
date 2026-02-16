package command

import (
	"testing"

	"github.com/TamtamHero/fw-fanctrl/internal/dto"
)

func TestParseSocketCommandUseWithOutputFormat(t *testing.T) {
	t.Parallel()

	parsed, err := ParseSocketCommand("--output-format JSON use lazy")
	if err != nil {
		t.Fatalf("ParseSocketCommand returned error: %v", err)
	}

	if parsed.Command != UseCommand {
		t.Fatalf("unexpected command: got %q want %q", parsed.Command, UseCommand)
	}

	if parsed.OutputFormat != dto.JSON {
		t.Fatalf("unexpected output format: got %q want %q", parsed.OutputFormat, dto.JSON)
	}

	if parsed.Args["strategy"] != "lazy" {
		t.Fatalf("unexpected strategy: got %q want %q", parsed.Args["strategy"], "lazy")
	}
}

func TestParseSocketCommandSetConfigQuotedJSON(t *testing.T) {
	t.Parallel()

	payload := `'{"defaultStrategy": "lazy", "strategyOnDischarging": "", "strategies": {"lazy": {"speedCurve": [{"temp": 0, "speed": 15}]}}}'`
	parsed, err := ParseSocketCommand("set_config " + payload)
	if err != nil {
		t.Fatalf("ParseSocketCommand returned error: %v", err)
	}

	if parsed.Command != SetConfigCommand {
		t.Fatalf("unexpected command: got %q want %q", parsed.Command, SetConfigCommand)
	}

	if parsed.Args["provided_config"] == "" {
		t.Fatal("expected provided_config to be populated")
	}
}

func TestParseSocketCommandRejectsUnknownFlag(t *testing.T) {
	t.Parallel()

	_, err := ParseSocketCommand("--unsupported print all")
	if err == nil {
		t.Fatal("expected parse error for unknown flag")
	}
}
