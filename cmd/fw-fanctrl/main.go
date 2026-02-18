package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/luis-c465/fw-fanctrl/internal/dto"
	"github.com/luis-c465/fw-fanctrl/internal/socket"
	"github.com/spf13/cobra"
)

const (
	defaultSocketPath = "/run/fw-fanctrl/.fw-fanctrl.commands.sock"
	defaultSocketType = "unix"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	var (
		socketController string
		outputFormat     string
	)

	rootCmd := &cobra.Command{
		Use:   "fw-fanctrl",
		Short: "Framework fan controller client",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	rootCmd.PersistentFlags().StringVar(&socketController, "socket-controller", defaultSocketType, "Socket controller type")
	rootCmd.PersistentFlags().StringVar(&socketController, "sc", defaultSocketType, "Socket controller type (shorthand)")
	rootCmd.PersistentFlags().StringVar(&outputFormat, "output-format", string(dto.Natural), "Output format (NATURAL or JSON)")

	rootCmd.PersistentPreRunE = func(_ *cobra.Command, _ []string) error {
		if socketController != defaultSocketType {
			return fmt.Errorf("unsupported socket controller %q (supported: %s)", socketController, defaultSocketType)
		}

		parsedOutputFormat := dto.ParseOutputFormat(outputFormat)
		if !strings.EqualFold(outputFormat, string(parsedOutputFormat)) {
			return fmt.Errorf("unsupported output format %q (supported: %s, %s)", outputFormat, dto.Natural, dto.JSON)
		}

		return nil
	}

	send := func(parts ...string) error {
		payload := buildSocketCommand(outputFormat, socketController, parts...)
		response, err := socket.SendCommand(defaultSocketPath, payload)
		if err != nil {
			return err
		}

		_, err = fmt.Fprintln(os.Stdout, response)
		return err
	}

	rootCmd.AddCommand(&cobra.Command{
		Use:   "use <strategy>",
		Short: "Change current strategy",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return send("use", args[0])
		},
	})

	rootCmd.AddCommand(&cobra.Command{
		Use:   "reset",
		Short: "Reset to default strategy",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return send("reset")
		},
	})

	rootCmd.AddCommand(&cobra.Command{
		Use:   "reload",
		Short: "Reload configuration",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return send("reload")
		},
	})

	rootCmd.AddCommand(&cobra.Command{
		Use:   "pause",
		Short: "Pause the service",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return send("pause")
		},
	})

	rootCmd.AddCommand(&cobra.Command{
		Use:   "resume",
		Short: "Resume the service",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return send("resume")
		},
	})

	rootCmd.AddCommand(&cobra.Command{
		Use:   "print [all|active|current|list|speed]",
		Short: "Print service information",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if len(args) == 0 {
				return send("print")
			}

			return send("print", args[0])
		},
	})

	rootCmd.AddCommand(&cobra.Command{
		Use:   "set_config <json>",
		Short: "Replace configuration",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return send("set_config", args[0])
		},
	})

	return rootCmd.Execute()
}

func buildSocketCommand(outputFormat string, socketController string, parts ...string) string {
	tokens := []string{"--output-format", outputFormat, "--socket-controller", socketController}
	tokens = append(tokens, parts...)

	encoded := make([]string, 0, len(tokens))
	for _, token := range tokens {
		encoded = append(encoded, strconv.Quote(token))
	}

	return strings.Join(encoded, " ")
}
