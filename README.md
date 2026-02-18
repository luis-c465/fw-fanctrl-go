# fw-fanctrl

`fw-fanctrl` controls Framework Laptop fan speed using configurable temperature/speed strategies.

This rewrite is implemented in Go and ships as two binaries:

- `fw-fanctrld`: daemon (service)
- `fw-fanctrl`: CLI client

There is no Python runtime dependency anymore.

## Requirements

- Linux kernel >= 6.11
- `ectool` installed and available in `PATH`
- Go >= 1.21 (only required when building from source)

## Installation

### From release tarball

1. Download and extract the release archive.
2. From the extracted directory, install:

```bash
sudo make install
sudo make enable
```

If `ectool` is not installed yet, install it first:

```bash
sudo make install-ectool
```

### From source

```bash
git clone "https://github.com/luis-c465/fw-fanctrl.git"
cd fw-fanctrl
make build
sudo make install
sudo make enable
```

## Update

Download the new release (or pull latest source) and run:

```bash
sudo make install
```

## Uninstall

```bash
sudo make disable
sudo make uninstall
```

## Quick Usage

```bash
fw-fanctrl print all
fw-fanctrl print list
fw-fanctrl use lazy
fw-fanctrl reset
fw-fanctrl --output-format JSON print current
```

## Migration Notes (Python -> Go)

- Existing config format is compatible (`/etc/fw-fanctrl/config.json` + `config.schema.json`).
- Socket protocol and path are unchanged, so third-party clients keep working.
- Service unit changed from `fw-fanctrl.service` to `fw-fanctrld.service`.
- Legacy Python CLI flags and `run` command are removed from the client.

Suggested upgrade path from old installs:

```bash
sudo systemctl stop fw-fanctrl || true
sudo systemctl disable fw-fanctrl || true
sudo make install
sudo make enable
```

## Documentation

- Commands: `doc/commands.md`
- Configuration: `doc/configuration.md`
- NixOS notes: `doc/nixos.md`

## Third-party projects

_Have a cool integration? Open a PR and add it here._

| Name | Description | Picture |
|---|---|---|
| [fw-fanctrl-gui](https://github.com/leopoldhub/fw-fanctrl-gui) | Simple customtkinter Python GUI with system tray for fw-fanctrl | [<img src="https://github.com/leopoldhub/fw-fanctrl-gui/blob/master/doc/screenshots/tray.png?raw=true" width="200">](https://github.com/leopoldhub/fw-fanctrl-gui) |
| [fw-fanctrl-revived-gnome-shell-extension](https://github.com/ghostdevv/fw-fanctrl-revived-gnome-shell-extension) | GNOME extension for changing fw-fanctrl strategy quickly | [<img src="https://raw.githubusercontent.com/ghostdevv/fw-fanctrl-revived-gnome-shell-extension/refs/heads/main/.github/example.png" width="200">](https://github.com/ghostdevv/fw-fanctrl-revived-gnome-shell-extension) |
| [fw_fanctrl_applet](https://github.com/not-a-feature/fw_fanctrl_applet) | Cinnamon applet to control strategy | [<img src="https://raw.githubusercontent.com/not-a-feature/fw_fanctrl_applet/main/screenshot.png" width="200">](https://github.com/not-a-feature/fw_fanctrl_applet) |
| [ulauncher-fw-fanctrl](https://github.com/ghostdevv/ulauncher-fw-fanctrl) | Ulauncher extension for fw-fanctrl commands | [<img src="https://raw.githubusercontent.com/ghostdevv/ulauncher-fw-fanctrl/32f7c0484b8903daa85f1b963ed4e901d7379a8a/.github/demo.png" width="200">](https://github.com/ghostdevv/ulauncher-fw-fanctrl) |

## Development

```bash
go test ./...
golangci-lint run ./...
```
