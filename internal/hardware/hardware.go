package hardware

type SensorReading struct {
	Name    string
	Index   int
	TempC   float64
	Present bool
}

type HardwareController interface {
	GetTemperature() (float64, error)
	GetTemperatures() ([]SensorReading, error)
	SetSpeed(speed int) error
	Pause() error
	Resume() error
	IsOnAC() (bool, error)
}
