# Commands

This project has two binaries:

- `fw-fanctrl`: client command you run manually
- `fw-fanctrld`: daemon (usually started by systemd)

## Client (`fw-fanctrl`)

Base form:

```bash
fw-fanctrl [global options] <command> [arguments]
```

Global options:

| Option | Optional | Choices | Default | Description |
|---|---|---|---|---|
| `--socket-controller`, `--sc` | yes | `unix` | `unix` | Socket controller used to communicate with daemon |
| `--output-format` | yes | `NATURAL`, `JSON` | `NATURAL` | Output format for command responses |

### `use <strategy>`

Change the current strategy.

### `reset`

Reset to default strategy behavior.

### `reload`

Reload configuration from disk.

### `pause`

Pause the service and return fan control to firmware.

### `resume`

Resume service fan control.

### `print [all|active|current|list|speed]`

Print service information (defaults to `all`).

| Choice | Description |
|---|---|
| `all` | Full runtime status |
| `active` | Whether controller is active |
| `current` | Strategy currently in use |
| `list` | Available strategy names |
| `speed` | Current fan speed percentage |

### `set_config <json>`

Replace configuration with a JSON payload.

Example:

```bash
fw-fanctrl set_config '{"$schema":"./config.schema.json","defaultStrategy":"lazy","strategyOnDischarging":"","strategies":{"lazy":{"fanSpeedUpdateFrequency":5,"movingAverageInterval":30,"speedCurve":[{"temp":0,"speed":15},{"temp":50,"speed":15},{"temp":65,"speed":25},{"temp":70,"speed":35},{"temp":75,"speed":50},{"temp":85,"speed":100}]}}}'
```

## Daemon (`fw-fanctrld`)

Base form:

```bash
fw-fanctrld [options] [strategy]
```

Options:

| Option | Optional | Choices | Default | Description |
|---|---|---|---|---|
| `--config`, `-c` | yes | file path | `/etc/fw-fanctrl/config.json` | Configuration file path |
| `--silent`, `-s` | yes | boolean flag | `false` | Disable periodic runtime output |
| `--hardware-controller`, `--hc` | yes | `ectool` | `ectool` | Hardware backend |
| `--socket-controller`, `--sc` | yes | `unix` | `unix` | Socket backend |
| `--no-battery-sensors` | yes | boolean flag | `false` | Ignore battery temp sensors |
| `--output-format` | yes | `NATURAL`, `JSON` | `JSON` | Runtime output format |
| `[strategy]` | yes | strategy name | config default | Initial strategy override |

Notes:

- `fw-fanctrl` no longer has a `run` subcommand.
- The daemon service name is `fw-fanctrld.service`.
