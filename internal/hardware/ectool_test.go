package hardware

import (
	"reflect"
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
