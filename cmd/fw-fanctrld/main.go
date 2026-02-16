package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/TamtamHero/fw-fanctrl/internal/command"
	"github.com/TamtamHero/fw-fanctrl/internal/config"
	"github.com/TamtamHero/fw-fanctrl/internal/controller"
	"github.com/TamtamHero/fw-fanctrl/internal/dto"
	"github.com/TamtamHero/fw-fanctrl/internal/hardware"
	"github.com/TamtamHero/fw-fanctrl/internal/socket"
	"github.com/spf13/cobra"
)

const (
	defaultConfigPath  = "/etc/fw-fanctrl/config.json"
	defaultSocketPath  = "/run/fw-fanctrl/.fw-fanctrl.commands.sock"
	defaultHardware    = "ectool"
	defaultSocketType  = "unix"
	defaultOutputStyle = "JSON"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	var (
		configPath         string
		silent             bool
		hardwareController string
		socketController   string
		noBatterySensors   bool
		outputFormat       string
	)

	rootCmd := &cobra.Command{
		Use:   "fw-fanctrld [strategy]",
		Short: "Framework fan controller daemon",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			strategyName := ""
			if len(args) == 1 {
				strategyName = args[0]
			}

			if hardwareController != defaultHardware {
				return fmt.Errorf("unsupported hardware controller %q (supported: %s)", hardwareController, defaultHardware)
			}

			if socketController != defaultSocketType {
				return fmt.Errorf("unsupported socket controller %q (supported: %s)", socketController, defaultSocketType)
			}

			if !strings.EqualFold(outputFormat, string(dto.Natural)) && !strings.EqualFold(outputFormat, string(dto.JSON)) {
				return fmt.Errorf("unsupported output format %q (supported: %s, %s)", outputFormat, dto.Natural, dto.JSON)
			}

			hw, err := hardware.NewEctoolHardwareController(noBatterySensors)
			if err != nil {
				return fmt.Errorf("failed to initialize hardware controller: %w", err)
			}

			cfg, err := config.NewConfiguration(configPath)
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			fanController := controller.NewFanController(hw, cfg, strategyName)
			commandHandler := command.NewCommandHandler(fanController)

			socketServer := socket.NewServer(defaultSocketPath, func(rawCommand string) (string, error) {
				parsedCommand, parseErr := command.ParseSocketCommand(rawCommand)
				if parseErr != nil {
					return command.FormatErrorOutput(parseErr, parsedCommand.OutputFormat), nil
				}

				response, handleErr := commandHandler.HandleCommand(
					parsedCommand.Command,
					parsedCommand.Args,
					parsedCommand.OutputFormat,
				)
				if handleErr != nil {
					return command.FormatErrorOutput(handleErr, parsedCommand.OutputFormat), nil
				}

				return response, nil
			})

			serverErrCh := make(chan error, 1)
			go func() {
				serverErrCh <- socketServer.Start()
			}()

			runErrCh := make(chan error, 1)
			go func() {
				runErrCh <- fanController.Run(!silent)
			}()

			signalCh := make(chan os.Signal, 1)
			signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)
			defer signal.Stop(signalCh)

			var cleanupOnce sync.Once
			cleanup := func() {
				cleanupOnce.Do(func() {
					if err := hw.Pause(); err != nil && !silent {
						fmt.Fprintf(os.Stderr, "failed to restore auto fan control: %v\n", err)
					}
					if err := socketServer.Stop(); err != nil && !silent {
						fmt.Fprintf(os.Stderr, "failed to stop socket server: %v\n", err)
					}
				})
			}

			for {
				select {
				case <-signalCh:
					cleanup()
					return nil
				case err := <-serverErrCh:
					cleanup()
					if err != nil {
						return fmt.Errorf("socket server exited: %w", err)
					}
					return nil
				case err := <-runErrCh:
					cleanup()
					if err != nil {
						return fmt.Errorf("controller loop exited: %w", err)
					}
					return nil
				}
			}
		},
	}

	rootCmd.Flags().StringVarP(&configPath, "config", "c", defaultConfigPath, "Path to configuration file")
	rootCmd.Flags().BoolVarP(&silent, "silent", "s", false, "Disable debug output")
	rootCmd.Flags().StringVar(&hardwareController, "hardware-controller", defaultHardware, "Hardware controller type")
	rootCmd.Flags().StringVar(&hardwareController, "hc", defaultHardware, "Hardware controller type (shorthand)")
	rootCmd.Flags().StringVar(&socketController, "socket-controller", defaultSocketType, "Socket controller type")
	rootCmd.Flags().StringVar(&socketController, "sc", defaultSocketType, "Socket controller type (shorthand)")
	rootCmd.Flags().BoolVar(&noBatterySensors, "no-battery-sensors", false, "Ignore battery temperature sensors")
	rootCmd.Flags().StringVar(&outputFormat, "output-format", defaultOutputStyle, "Runtime output format (NATURAL or JSON)")

	if err := rootCmd.Execute(); err != nil {
		return err
	}

	return nil
}
