package command

import (
	"fmt"
	"strings"

	"github.com/luis-c465/fw-fanctrl/internal/dto"
)

type ParsedSocketCommand struct {
	Command      string
	Args         map[string]string
	OutputFormat dto.OutputFormat
}

func ParseSocketCommand(raw string) (ParsedSocketCommand, error) {
	parsed := ParsedSocketCommand{
		Args:         map[string]string{},
		OutputFormat: dto.Natural,
	}

	tokens, err := shellSplit(raw)
	if err != nil {
		return parsed, fmt.Errorf("failed to parse command payload: %w", err)
	}

	if len(tokens) == 0 {
		return parsed, fmt.Errorf("received empty command")
	}

	idx := 0
	for idx < len(tokens) && strings.HasPrefix(tokens[idx], "-") {
		token := tokens[idx]
		switch {
		case token == "--output-format":
			if idx+1 >= len(tokens) {
				return parsed, fmt.Errorf("missing value for --output-format")
			}
			parsed.OutputFormat = dto.ParseOutputFormat(tokens[idx+1])
			idx += 2
		case strings.HasPrefix(token, "--output-format="):
			parsed.OutputFormat = dto.ParseOutputFormat(strings.TrimPrefix(token, "--output-format="))
			idx++
		case token == "--socket-controller" || token == "--sc":
			if idx+1 >= len(tokens) {
				return parsed, fmt.Errorf("missing value for %s", token)
			}
			idx += 2
		case strings.HasPrefix(token, "--socket-controller="):
			idx++
		default:
			return parsed, fmt.Errorf("unsupported flag: %s", token)
		}
	}

	if idx >= len(tokens) {
		return parsed, fmt.Errorf("missing command")
	}

	parsed.Command = tokens[idx]
	idx++

	switch parsed.Command {
	case UseCommand:
		if idx >= len(tokens) {
			return parsed, fmt.Errorf("missing strategy argument")
		}
		parsed.Args["strategy"] = tokens[idx]
		idx++
		if idx != len(tokens) {
			return parsed, fmt.Errorf("too many arguments for %q", UseCommand)
		}
	case ResetCommand, ReloadCommand, PauseCommand, ResumeCommand:
		if idx != len(tokens) {
			return parsed, fmt.Errorf("too many arguments for %q", parsed.Command)
		}
	case PrintCommand:
		selection := "all"
		if idx < len(tokens) {
			selection = tokens[idx]
			idx++
		}
		if idx != len(tokens) {
			return parsed, fmt.Errorf("too many arguments for %q", PrintCommand)
		}
		parsed.Args["print_selection"] = selection
	case SetConfigCommand:
		if idx >= len(tokens) {
			return parsed, fmt.Errorf("missing configuration payload")
		}
		parsed.Args["provided_config"] = strings.Join(tokens[idx:], " ")
	default:
		return parsed, fmt.Errorf("unknown command: %q", parsed.Command)
	}

	return parsed, nil
}

func shellSplit(raw string) ([]string, error) {
	args := []string{}
	var token strings.Builder

	inSingle := false
	inDouble := false
	escaped := false

	flush := func() {
		if token.Len() == 0 {
			return
		}
		args = append(args, token.String())
		token.Reset()
	}

	for _, r := range raw {
		if escaped {
			token.WriteRune(r)
			escaped = false
			continue
		}

		if inSingle {
			if r == '\'' {
				inSingle = false
				continue
			}
			token.WriteRune(r)
			continue
		}

		if inDouble {
			switch r {
			case '"':
				inDouble = false
			case '\\':
				escaped = true
			default:
				token.WriteRune(r)
			}
			continue
		}

		switch r {
		case '\\':
			escaped = true
		case '\'', '"':
			if r == '\'' {
				inSingle = true
			} else {
				inDouble = true
			}
		case ' ', '\t', '\n', '\r':
			flush()
		default:
			token.WriteRune(r)
		}
	}

	if escaped || inSingle || inDouble {
		return nil, fmt.Errorf("unterminated quoted or escaped sequence")
	}

	flush()
	return args, nil
}
