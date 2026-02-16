package config

type SpeedCurvePoint struct {
	Temp  float64 `json:"temp"`
	Speed int     `json:"speed"`
}

type StrategyParams struct {
	FanSpeedUpdateFrequency int               `json:"fanSpeedUpdateFrequency"`
	MovingAverageInterval   int               `json:"movingAverageInterval"`
	SpeedCurve              []SpeedCurvePoint `json:"speedCurve"`
}

type Strategy struct {
	Name                    string
	FanSpeedUpdateFrequency int
	MovingAverageInterval   int
	SpeedCurve              []SpeedCurvePoint
}

func NewStrategy(name string, params StrategyParams) Strategy {
	params = params.WithDefaults()

	return Strategy{
		Name:                    name,
		FanSpeedUpdateFrequency: params.FanSpeedUpdateFrequency,
		MovingAverageInterval:   params.MovingAverageInterval,
		SpeedCurve:              params.SpeedCurve,
	}
}

func (s Strategy) IsDefault(defaultStrategyName string) bool {
	return s.Name == defaultStrategyName
}

func (p StrategyParams) WithDefaults() StrategyParams {
	if p.FanSpeedUpdateFrequency == 0 {
		p.FanSpeedUpdateFrequency = 5
	}

	if p.MovingAverageInterval == 0 {
		p.MovingAverageInterval = 20
	}

	return p
}
