package config

import "fmt"

type ConfigurationParsingError struct {
	Message string
}

func (e ConfigurationParsingError) Error() string {
	return e.Message
}

type InvalidStrategyError struct {
	StrategyName string
}

func (e InvalidStrategyError) Error() string {
	return fmt.Sprintf("invalid strategy: %s", e.StrategyName)
}
