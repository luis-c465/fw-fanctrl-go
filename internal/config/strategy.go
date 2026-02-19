package config

type SpeedCurvePoint struct {
	Temp  float64 `json:"temp"`
	Speed int     `json:"speed"`
}

type SensorCurve struct {
	Name                  string            `json:"name"`
	Sensors               []string          `json:"sensors"`
	MovingAverageInterval int               `json:"movingAverageInterval,omitempty"`
	SpeedCurve            []SpeedCurvePoint `json:"speedCurve"`
}

type StrategyParams struct {
	FanSpeedUpdateFrequency int               `json:"fanSpeedUpdateFrequency"`
	MovingAverageInterval   int               `json:"movingAverageInterval"`
	SpeedCurve              []SpeedCurvePoint `json:"speedCurve,omitempty"`
	SensorCurves            []SensorCurve     `json:"sensorCurves,omitempty"`
}

type Strategy struct {
	Name                    string
	FanSpeedUpdateFrequency int
	MovingAverageInterval   int
	SpeedCurve              []SpeedCurvePoint
	SensorCurves            []SensorCurve
}

func NewStrategy(name string, params StrategyParams) Strategy {
	params = params.WithDefaults()

	return Strategy{
		Name:                    name,
		FanSpeedUpdateFrequency: params.FanSpeedUpdateFrequency,
		MovingAverageInterval:   params.MovingAverageInterval,
		SpeedCurve:              params.SpeedCurve,
		SensorCurves:            params.SensorCurves,
	}
}

func (s Strategy) IsDefault(defaultStrategyName string) bool {
	return s.Name == defaultStrategyName
}

func (s Strategy) IsMultiSensor() bool {
	return len(s.SensorCurves) > 0
}

func (p StrategyParams) WithDefaults() StrategyParams {
	if p.FanSpeedUpdateFrequency == 0 {
		p.FanSpeedUpdateFrequency = 5
	}

	if p.MovingAverageInterval == 0 {
		p.MovingAverageInterval = 20
	}

	if len(p.SensorCurves) > 0 {
		for i := range p.SensorCurves {
			if p.SensorCurves[i].MovingAverageInterval == 0 {
				p.SensorCurves[i].MovingAverageInterval = p.MovingAverageInterval
			}
		}
	}

	return p
}
