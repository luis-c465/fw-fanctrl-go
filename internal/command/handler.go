package command

import (
	"fmt"
	"strconv"

	"github.com/TamtamHero/fw-fanctrl/internal/controller"
	"github.com/TamtamHero/fw-fanctrl/internal/dto"
)

const (
	UseCommand       = "use"
	ResetCommand     = "reset"
	ReloadCommand    = "reload"
	PauseCommand     = "pause"
	ResumeCommand    = "resume"
	PrintCommand     = "print"
	SetConfigCommand = "set_config"
)

type CommandHandler struct {
	fanController *controller.FanController
}

func NewCommandHandler(fanController *controller.FanController) *CommandHandler {
	return &CommandHandler{fanController: fanController}
}

func (h *CommandHandler) HandleCommand(command string, args map[string]string, outputFormat dto.OutputFormat) (string, error) {
	if args == nil {
		args = map[string]string{}
	}

	switch command {
	case ResetCommand:
		h.fanController.ClearOverwrittenStrategy()

		currentStrategy, err := h.fanController.GetCurrentStrategy()
		if err != nil {
			return "", err
		}

		result := dto.NewStrategyResetCommandResult(currentStrategy.Name, h.fanController.IsDefaultStrategyInUse())
		return result.ToOutputFormat(outputFormat), nil
	case UseCommand:
		strategyName := args["strategy"]
		if strategyName == "" {
			return "", fmt.Errorf("missing strategy argument")
		}

		if strategyName == "defaultStrategy" {
			h.fanController.ClearOverwrittenStrategy()

			currentStrategy, err := h.fanController.GetCurrentStrategy()
			if err != nil {
				return "", err
			}

			result := dto.NewStrategyResetCommandResult(currentStrategy.Name, h.fanController.IsDefaultStrategyInUse())
			return result.ToOutputFormat(outputFormat), nil
		}

		if err := h.fanController.OverwriteStrategy(strategyName); err != nil {
			return "", err
		}

		currentStrategy, err := h.fanController.GetCurrentStrategy()
		if err != nil {
			return "", err
		}

		result := dto.NewStrategyChangeCommandResult(currentStrategy.Name, h.fanController.IsDefaultStrategyInUse())
		return result.ToOutputFormat(outputFormat), nil
	case ReloadCommand:
		if err := h.fanController.ReloadConfiguration(); err != nil {
			return "", err
		}

		currentStrategy, err := h.fanController.GetCurrentStrategy()
		if err != nil {
			return "", err
		}

		result := dto.NewConfigurationReloadCommandResult(currentStrategy.Name, h.fanController.IsDefaultStrategyInUse())
		return result.ToOutputFormat(outputFormat), nil
	case PauseCommand:
		if err := h.fanController.Pause(); err != nil {
			return "", err
		}

		result := dto.NewServicePauseCommandResult()
		return result.ToOutputFormat(outputFormat), nil
	case ResumeCommand:
		if err := h.fanController.Resume(); err != nil {
			return "", err
		}

		currentStrategy, err := h.fanController.GetCurrentStrategy()
		if err != nil {
			return "", err
		}

		result := dto.NewServiceResumeCommandResult(currentStrategy.Name, h.fanController.IsDefaultStrategyInUse())
		return result.ToOutputFormat(outputFormat), nil
	case PrintCommand:
		return h.handlePrintCommand(args, outputFormat)
	case SetConfigCommand:
		providedConfig := args["provided_config"]
		if providedConfig == "" {
			return "", fmt.Errorf("missing provided configuration payload")
		}

		if err := h.fanController.SetConfiguration([]byte(providedConfig)); err != nil {
			return "", err
		}

		currentStrategy, err := h.fanController.GetCurrentStrategy()
		if err != nil {
			return "", err
		}

		result := dto.NewSetConfigurationCommandResult(
			currentStrategy.Name,
			h.fanController.ConfigurationForOutput(),
			h.fanController.IsDefaultStrategyInUse(),
		)
		return result.ToOutputFormat(outputFormat), nil
	default:
		return "", fmt.Errorf("unknown command: %q", command)
	}
}

func FormatErrorOutput(err error, outputFormat dto.OutputFormat) string {
	if err == nil {
		return ""
	}

	result := dto.NewErrorCommandResult(fmt.Sprintf("An error occurred while treating a socket command: %v", err))
	return result.ToOutputFormat(outputFormat)
}

func (h *CommandHandler) handlePrintCommand(args map[string]string, outputFormat dto.OutputFormat) (string, error) {
	selection := args["print_selection"]
	if selection == "" {
		selection = "all"
	}

	switch selection {
	case "all":
		snapshot, err := h.fanController.StatusSnapshot()
		if err != nil {
			return "", err
		}

		result := dto.NewStatusRuntimeResult(
			snapshot.Strategy,
			snapshot.Default,
			snapshot.Speed,
			snapshot.Temperature,
			snapshot.MovingAverageTemperature,
			snapshot.EffectiveTemperature,
			snapshot.Active,
			snapshot.Configuration,
		)
		return result.ToOutputFormat(outputFormat), nil
	case "active":
		result := dto.NewPrintActiveCommandResult(h.fanController.IsActive())
		return result.ToOutputFormat(outputFormat), nil
	case "current":
		currentStrategy, err := h.fanController.GetCurrentStrategy()
		if err != nil {
			return "", err
		}

		result := dto.NewPrintCurrentStrategyCommandResult(currentStrategy.Name, h.fanController.IsDefaultStrategyInUse())
		return result.ToOutputFormat(outputFormat), nil
	case "list":
		result := dto.NewPrintStrategyListCommandResult(h.fanController.GetStrategies())
		return result.ToOutputFormat(outputFormat), nil
	case "speed":
		result := dto.NewPrintFanSpeedCommandResult(strconv.Itoa(h.fanController.GetSpeed()))
		return result.ToOutputFormat(outputFormat), nil
	default:
		return "", fmt.Errorf("invalid print selection: %q", selection)
	}
}
