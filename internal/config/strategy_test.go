package config

import "testing"

func TestStrategyDefaultsFanSpeedUpdateFrequency(t *testing.T) {
	t.Parallel()

	strategy := NewStrategy("test", StrategyParams{
		FanSpeedUpdateFrequency: 0,
		MovingAverageInterval:   10,
		SpeedCurve:              []SpeedCurvePoint{{Temp: 0, Speed: 10}},
	})

	if strategy.FanSpeedUpdateFrequency != 5 {
		t.Fatalf("expected default fanSpeedUpdateFrequency 5, got %d", strategy.FanSpeedUpdateFrequency)
	}
}

func TestStrategyDefaultsMovingAverageInterval(t *testing.T) {
	t.Parallel()

	strategy := NewStrategy("test", StrategyParams{
		FanSpeedUpdateFrequency: 4,
		MovingAverageInterval:   0,
		SpeedCurve:              []SpeedCurvePoint{{Temp: 0, Speed: 10}},
	})

	if strategy.MovingAverageInterval != 20 {
		t.Fatalf("expected default movingAverageInterval 20, got %d", strategy.MovingAverageInterval)
	}
}

func TestStrategyPreservesSpeedCurve(t *testing.T) {
	t.Parallel()

	expected := []SpeedCurvePoint{
		{Temp: 0, Speed: 15},
		{Temp: 65, Speed: 25},
	}

	strategy := NewStrategy("test", StrategyParams{
		FanSpeedUpdateFrequency: 4,
		MovingAverageInterval:   10,
		SpeedCurve:              expected,
	})

	if len(strategy.SpeedCurve) != len(expected) {
		t.Fatalf("expected %d speed curve points, got %d", len(expected), len(strategy.SpeedCurve))
	}

	for i := range expected {
		if strategy.SpeedCurve[i] != expected[i] {
			t.Fatalf("speed curve mismatch at index %d: expected %+v, got %+v", i, expected[i], strategy.SpeedCurve[i])
		}
	}
}

func TestStrategyIsDefault(t *testing.T) {
	t.Parallel()

	strategy := NewStrategy("lazy", StrategyParams{
		FanSpeedUpdateFrequency: 5,
		MovingAverageInterval:   20,
		SpeedCurve:              []SpeedCurvePoint{{Temp: 0, Speed: 15}},
	})

	if !strategy.IsDefault("lazy") {
		t.Fatal("expected strategy to match default name")
	}

	if strategy.IsDefault("medium") {
		t.Fatal("expected strategy not to match different default name")
	}
}

func TestStrategyIsMultiSensor(t *testing.T) {
	t.Parallel()

	strategy := NewStrategy("multi", StrategyParams{
		FanSpeedUpdateFrequency: 5,
		MovingAverageInterval:   30,
		SensorCurves: []SensorCurve{
			{
				Name:    "cpu",
				Sensors: []string{"cpu@4c"},
				SpeedCurve: []SpeedCurvePoint{
					{Temp: 0, Speed: 15},
					{Temp: 85, Speed: 100},
				},
			},
		},
	})

	if !strategy.IsMultiSensor() {
		t.Fatal("expected strategy to be in multi-sensor mode")
	}
}

func TestStrategyDefaultsSensorCurveMovingAverageInterval(t *testing.T) {
	t.Parallel()

	strategy := NewStrategy("multi", StrategyParams{
		FanSpeedUpdateFrequency: 5,
		MovingAverageInterval:   30,
		SensorCurves: []SensorCurve{
			{
				Name:    "cpu",
				Sensors: []string{"cpu@4c"},
				SpeedCurve: []SpeedCurvePoint{
					{Temp: 0, Speed: 15},
					{Temp: 85, Speed: 100},
				},
			},
		},
	})

	if strategy.SensorCurves[0].MovingAverageInterval != 30 {
		t.Fatalf("expected sensor curve movingAverageInterval 30, got %d", strategy.SensorCurves[0].MovingAverageInterval)
	}
}
