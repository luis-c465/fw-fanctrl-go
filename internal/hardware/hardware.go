package hardware

type HardwareController interface {
	GetTemperature() (float64, error)
	SetSpeed(speed int) error
	Pause() error
	Resume() error
	IsOnAC() (bool, error)
}
