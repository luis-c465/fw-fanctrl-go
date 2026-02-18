# fw-fanctrl Rewrite Plan: Python → Go

---

## Section 1: High-Level Overview

### 1.1 — Goal Statement

Rewrite the `fw-fanctrl` Framework Laptop fan controller from Python to Go, producing two static binaries (`fw-fanctrld` daemon and `fw-fanctrl` CLI client) that communicate over Unix domain sockets. The rewrite eliminates the Python 3.12 runtime dependency, simplifies installation to a single binary copy, and adds unit test coverage for core logic — all while maintaining full behavioral compatibility with the existing service (same config format, same socket protocol, same systemd integration).

### 1.2 — Approach Summary

- **Language:** Go (selected for its excellent stdlib coverage of Unix sockets, JSON, subprocess execution, and CLI argument parsing — all core needs of this project — plus trivial cross-compilation and single static binary output)
- **Architecture:** Two binaries from a single Go module:
  - `fw-fanctrld` — the daemon that reads temperature, controls fan speed, listens on a Unix socket for commands
  - `fw-fanctrl` — the CLI client that sends commands to the daemon via the Unix socket
- **Key libraries:**
  - `encoding/json` (stdlib) — config parsing and JSON output format
  - `github.com/kaptinlin/jsonschema` — JSON Schema Draft 2020-12 validation (matching the existing `config.schema.json`)
  - `github.com/spf13/cobra` — CLI argument parsing with subcommands (replaces Python's argparse)
  - `net` (stdlib) — Unix domain socket client/server
  - `os/exec` (stdlib) — subprocess calls to `ectool`
  - `embed` (stdlib) — compile-time embedding of default config and schema files
- **Config format:** 100% backward compatible — same `config.json` and `config.schema.json` files
- **Socket protocol:** Same text-based protocol over `AF_UNIX` `SOCK_STREAM` at `/run/fw-fanctrl/.fw-fanctrl.commands.sock`
- **Legacy CLI dropped:** Only the new subcommand-based CLI is implemented
- **Installation:** Makefile-based build and install, replacing the Python-specific `install.sh`

### 1.3 — Decisions Log

- **Decision:** Use Go over Rust
  - **Alternatives considered:** Rust (stronger safety guarantees, zero-cost abstractions), C (maximum control, minimal dependencies)
  - **Rationale:** Go's stdlib covers 90% of this project's needs (JSON, sockets, subprocesses, embedding). The project's complexity doesn't warrant Rust's steeper learning curve. Go produces static binaries with trivial cross-compilation.

- **Decision:** Two separate binaries (`fw-fanctrld` + `fw-fanctrl`)
  - **Alternatives considered:** Single binary with subcommand routing (current Python design)
  - **Rationale:** Cleaner separation of concerns — daemon code doesn't ship in the CLI binary and vice versa. Smaller attack surface for the privileged daemon.

- **Decision:** Use `github.com/spf13/cobra` for CLI parsing
  - **Alternatives considered:** `flag` (stdlib, too basic for subcommands), `github.com/urfave/cli` (similar capability, less popular), hand-rolled parser
  - **Rationale:** Cobra is the de facto standard for Go CLIs with subcommands. Excellent help generation, shell completion support, and well-maintained.

- **Decision:** Use `github.com/kaptinlin/jsonschema` for JSON Schema validation
  - **Alternatives considered:** `github.com/xeipuuv/gojsonschema` (popular but Draft-07 focused), `github.com/santhosh-tekuri/jsonschema` (good but API less ergonomic), custom validation (loses schema file)
  - **Rationale:** Native Draft 2020-12 support matching the existing schema. Clean API. Actively maintained.

- **Decision:** Drop legacy CLI compatibility
  - **Alternatives considered:** Port the legacy `--run`, `--query`, `--reload` flag-based parser
  - **Rationale:** The legacy parser was already printing deprecation warnings. Clean break for the rewrite.

- **Decision:** Makefile-based build/install
  - **Alternatives considered:** Rewritten `install.sh`, `goreleaser`
  - **Rationale:** Idiomatic for compiled Go projects. Can handle build, install, uninstall, and ectool fetching.

- **Decision:** New repository
  - **Alternatives considered:** New branch in existing repo, coexistence
  - **Rationale:** Clean break from Python codebase.

### 1.4 — Assumptions & Open Questions

**Assumptions:**
- The socket protocol between daemon and CLI is a simple text-based request/response: the client sends the full command string, the daemon parses it, executes, and sends back a text or JSON response. This is preserved exactly.
- The `ectool` binary remains an external dependency called via subprocess — we are NOT rewriting ectool interaction as a library.
- The daemon runs as root (required for `ectool` access and socket creation in `/run/`).
- Go 1.21+ is the minimum Go version (for `embed`, generics, `slog`).
- The `config.schema.json` file's `$schema` pattern constraint (`^\./config\.schema\.json$`) may need adjustment since the schema will be embedded — the validator should still work with the embedded schema bytes.

**Open Questions (non-blocking):**
- Should the new repo be under the same GitHub org (`luis-c465`) or a different one? (Doesn't affect the plan)
- Should `goreleaser` be used for release automation in addition to the Makefile? (Can be added later)
- Should the NixOS packaging branch be considered in this plan? (Out of scope — can be done after the rewrite)

### 1.5 — Risk Register

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| JSON Schema validation library doesn't handle the existing `config.schema.json` correctly (e.g., `oneOf`, `$ref`, `patternProperties`) | Medium | High | Write a dedicated test that validates the existing `config.json` against the embedded `config.schema.json` using the chosen library. Test early in Step 4. If it fails, fall back to `santhosh-tekuri/jsonschema` or custom validation. |
| Socket protocol incompatibility breaks existing GUI/extension clients (fw-fanctrl-gui, GNOME extension, etc.) | Medium | High | The socket protocol is simple text: send command string, receive result string. Ensure byte-for-byte compatible output format for both NATURAL and JSON modes. Write integration tests comparing output. |
| `ectool` output format changes between versions, breaking regex parsing | Low | High | Port the exact same regex patterns from the Python code. The `ectool` binary is pinned to a specific build (job #899). |
| Race conditions in concurrent socket command handling (Python used GIL for implicit safety) | Medium | Medium | Use Go's `sync.Mutex` to protect shared state (speed, active, overwritten_strategy, temp_history, timecount). The Python code relied on the GIL; Go requires explicit synchronization. |
| Makefile doesn't cover all installation edge cases that `install.sh` handled (dest-dir, prefix-dir, sysconf-dir, pipx, etc.) | Low | Medium | The Makefile will support `DESTDIR`, `PREFIX`, and `SYSCONFDIR` variables, covering the main use cases. Exotic options like `--pipx` and `--python-prefix-dir` are Python-specific and no longer needed. |
| Embedded default config diverges from installed config over time | Low | Low | The embedded config is only used as a fallback when no config file exists at the expected path. Same behavior as the Python version. |

### 1.6 — Step Sequence Overview

1. **Initialize Go module and project structure** — scaffold the repository with standard Go layout
2. **Implement configuration and strategy types** — config parsing, JSON Schema validation, strategy model
3. **Implement hardware controller (ectool)** — subprocess calls to ectool for temperature, fan speed, battery status
4. **Implement fan controller core logic** — temperature averaging, speed curve interpolation, main control loop
5. **Implement command handling and DTO types** — command dispatch, result formatting (NATURAL + JSON)
6. **Implement daemon binary (`fw-fanctrld`)** — main entrypoint, CLI flags, signal handling, systemd integration
7. **Implement CLI client binary (`fw-fanctrl`)** — client socket, cobra commands, output formatting
8. **Implement unit tests** — tests for config parsing, strategy selection, speed interpolation, temperature averaging
9. **Implement Makefile and installation assets** — build, install, uninstall targets, systemd service file, suspend hook
10. **Implement CI/CD workflows** — GitHub Actions for testing, linting, cross-compiled release builds
11. **Update documentation** — README, commands reference, configuration guide

### 1.7 — Dependency Graph

```
Step 1 (scaffold)
  ├── Step 2 (config/strategy) ──┐
  └── Step 3 (hardware/ectool) ──┤
                                  ├── Step 4 (fan controller core)
                                  │     └── Step 5 (DTOs + command handling)
                                  │           ├── Step 6 (daemon binary)
                                  │           └── Step 7 (CLI client binary)
                                  │                 ├── Step 8 (unit tests)     ← parallel
                                  │                 ├── Step 9 (Makefile)       ← parallel
                                  │                 └── Step 11 (docs)         ← parallel
                                  │                       └── Step 10 (CI/CD)
```

**Key parallelization opportunities:**
- Steps 2 and 3 can be done in parallel (config vs hardware — no dependencies on each other)
- Steps 8, 9, and 11 can all be done in parallel (tests, build system, docs are independent)

**Critical path:** Steps 1 → 2 → 4 → 5 → 6 → 7 → 9 → 10 (8 sequential steps)

---

## Section 2: Step-by-Step Execution Plan

---

### Step 1: Initialize Go Module and Project Structure

**Objective:** Create the new repository with the standard Go project layout, Go module initialization, and all placeholder directories/files.

**Context:**
- This is the first step. Starting from an empty repository.
- Establishes the foundation that all subsequent steps build upon.

**Scope:**
- Create the following directory structure:
  ```
  fw-fanctrl/
  ├── cmd/
  │   ├── fw-fanctrl/        # CLI client binary
  │   │   └── main.go
  │   └── fw-fanctrld/       # Daemon binary
  │       └── main.go
  ├── internal/
  │   ├── config/             # Configuration parsing and validation
  │   ├── controller/         # Fan controller core logic
  │   ├── hardware/           # Hardware controller interface + ectool impl
  │   ├── socket/             # Unix socket server and client
  │   ├── command/            # Command parsing and result types
  │   └── dto/                # Output formatting types (command results, runtime results)
  ├── resources/
  │   ├── config.json         # Default config (embedded at compile time)
  │   └── config.schema.json  # JSON Schema (embedded at compile time)
  ├── services/
  │   ├── fw-fanctrld.service # systemd service file (template)
  │   └── system-sleep/
  │       └── fw-fanctrl-suspend  # suspend/resume hook (template)
  ├── fetch/
  │   └── ectool/
  │       └── linux/
  │           ├── gitlab_job_id
  │           └── hash.sha256
  ├── Makefile
  ├── go.mod
  ├── go.sum
  ├── .gitignore
  ├── .editorconfig
  ├── LICENSE
  └── README.md
  ```

**Sub-tasks:**
1. Create a new directory (or `git init` a new repo) named `fw-fanctrl`.
2. Run `go mod init github.com/luis-c465/fw-fanctrl` (or appropriate module path) to create `go.mod`. Set Go version to `1.21` minimum.
3. Create all directories listed above: `cmd/fw-fanctrl/`, `cmd/fw-fanctrld/`, `internal/config/`, `internal/controller/`, `internal/hardware/`, `internal/socket/`, `internal/command/`, `internal/dto/`, `resources/`, `services/`, `services/system-sleep/`, `fetch/ectool/linux/`.
4. Create placeholder `main.go` files in both `cmd/` directories with `package main` and an empty `func main()`.
5. Copy `config.json` and `config.schema.json` from the original project's `src/fw_fanctrl/_resources/` into `resources/`. These files are identical — no modifications needed.
6. Copy `fetch/ectool/linux/gitlab_job_id` and `fetch/ectool/linux/hash.sha256` from the original project.
7. Copy the `LICENSE` file from the original project.
8. Copy `.editorconfig` from the original project.
9. Create a `.gitignore` appropriate for Go projects (ignore `bin/`, `dist/`, `.idea/`, `.vscode/`, `*.exe`, etc.).
10. Add initial Go dependencies: `go get github.com/spf13/cobra` and `go get github.com/kaptinlin/jsonschema`.

**Edge Cases & Gotchas:**
- Ensure the Go module path matches the intended GitHub repository URL.
- The `resources/` directory must be at the module root so `go:embed` directives can reference it from any package using relative paths from the embedding file's location. The embed directives will be placed in the `internal/config/` package.

**Verification:**
- `go build ./...` succeeds with no errors.
- Directory structure matches the specification above.
- `go mod tidy` runs cleanly.

**Depends On:** None
**Blocks:** All subsequent steps

---

### Step 2: Implement Configuration and Strategy Types

**Objective:** Implement the configuration loading, JSON Schema validation, strategy model, and embedded default resources — the data layer that everything else depends on.

**Context:**
- Step 1 has established the project structure with placeholder files and embedded resources.
- This step implements the equivalent of `Configuration.py`, `Strategy.py`, and the JSON Schema validation from the Python project.

**Scope:**
- Files to create:
  - `internal/config/embed.go` — `go:embed` directives for `config.json` and `config.schema.json`
  - `internal/config/config.go` — `Configuration` struct, `Load()`, `Parse()`, `Reload()`, `Save()`, `GetStrategy()`, `GetStrategies()`, `GetDefaultStrategy()`, `GetDischargingStrategy()`
  - `internal/config/strategy.go` — `Strategy` struct with `Name`, `FanSpeedUpdateFrequency`, `MovingAverageInterval`, `SpeedCurve` fields
  - `internal/config/errors.go` — custom error types: `ConfigurationParsingError`, `InvalidStrategyError`

**Sub-tasks:**
1. **Create `internal/config/embed.go`:**
   - Use `//go:embed` to embed `resources/config.json` as `DefaultConfigJSON []byte` and `resources/config.schema.json` as `ConfigSchemaJSON []byte`.
   - Note: The embed directive path is relative to the file containing it. Since this file is in `internal/config/`, the embed path needs to reference `../../resources/config.json`. Alternatively, place the embed in a top-level `resources.go` file and export the variables — this is cleaner.
   - **Revised approach:** Create `resources/embed.go` with `package resources` containing the embed directives, then import from `internal/config/`. OR, simpler: place a single `embed.go` at the module root in a `resources` package.

2. **Create `internal/config/strategy.go`:**
   - Define `SpeedCurvePoint` struct: `Temp float64 \`json:"temp"\``, `Speed int \`json:"speed"\``
   - Define `StrategyParams` struct: `FanSpeedUpdateFrequency int \`json:"fanSpeedUpdateFrequency"\``, `MovingAverageInterval int \`json:"movingAverageInterval"\``, `SpeedCurve []SpeedCurvePoint \`json:"speedCurve"\``
   - Define `Strategy` struct: `Name string`, `Params StrategyParams` (or flatten the fields)
   - Apply defaults: if `FanSpeedUpdateFrequency` is 0, default to 5. If `MovingAverageInterval` is 0, default to 20. Match the Python behavior exactly.

3. **Create `internal/config/errors.go`:**
   - Define `ConfigurationParsingError` struct implementing `error` interface with a `Message` field.
   - Define `InvalidStrategyError` struct implementing `error` interface with a `StrategyName` field.

4. **Create `internal/config/config.go`:**
   - Define the raw config struct for JSON unmarshaling:
     ```go
     type RawConfig struct {
         Schema               string                       `json:"$schema"`
         DefaultStrategy      string                       `json:"defaultStrategy"`
         StrategyOnDischarging string                      `json:"strategyOnDischarging"`
         Strategies           map[string]StrategyParams    `json:"strategies"`
     }
     ```
   - Define `Configuration` struct with fields: `Path string`, `Data RawConfig`.
   - Implement `NewConfiguration(path string) (*Configuration, error)` — calls `Reload()`.
   - Implement `Parse(rawJSON []byte) (RawConfig, error)`:
     - Unmarshal JSON into `RawConfig`.
     - If `$schema` field is missing, inject it from the default config.
     - Validate against the embedded JSON Schema using `kaptinlin/jsonschema`:
       - Create a `jsonschema.NewCompiler()`, compile the embedded schema bytes, then call `schema.Validate(rawJSON)`.
     - Check that `DefaultStrategy` exists in `Strategies` map.
     - Check that `StrategyOnDischarging` is either empty string or exists in `Strategies` map.
     - Return parsed config or appropriate error.
   - Implement `Reload() error`:
     - If config file doesn't exist at `Path`, copy the embedded default config to that path (create parent directories if needed).
     - Read the file, call `Parse()`, store result in `Data`.
   - Implement `Save() error` — marshal `Data` to indented JSON, write to `Path`.
   - Implement `GetStrategies() []string` — return keys of `Data.Strategies`.
   - Implement `GetStrategy(name string) (Strategy, error)`:
     - Handle special names: `"strategyOnDischarging"` → resolve to actual strategy name (or default if empty). `"defaultStrategy"` → resolve to actual strategy name.
     - Look up in `Data.Strategies` map. Return `InvalidStrategyError` if not found.
     - Construct and return `Strategy` with defaults applied.
   - Implement `GetDefaultStrategy() (Strategy, error)` — calls `GetStrategy("defaultStrategy")`.
   - Implement `GetDischargingStrategy() (Strategy, error)` — calls `GetStrategy("strategyOnDischarging")`.

**Edge Cases & Gotchas:**
- The `$schema` field in config.json has a pattern constraint `^\./config\.schema\.json$`. When validating embedded config, this should still pass since we're validating the JSON content, not the file path resolution.
- The `go:embed` directive requires the embedded files to be in or below the directory of the Go source file. The cleanest approach is to have `resources/embed.go` as a separate package.
- JSON Schema `oneOf` for `strategyOnDischarging` (either empty string or valid strategy key) must be handled correctly by the validation library.
- When copying default config to disk on first run, ensure parent directory `/etc/fw-fanctrl/` exists (create with `os.MkdirAll`).
- The `StrategyParams` JSON fields use camelCase (`fanSpeedUpdateFrequency`) matching the config file format.

**Verification:**
- Write a test in `internal/config/config_test.go` that:
  - Parses the embedded default `config.json` and verifies it passes validation.
  - Verifies all 7 default strategies are loaded correctly.
  - Verifies `GetDefaultStrategy()` returns "lazy".
  - Verifies `GetDischargingStrategy()` returns the default strategy when `strategyOnDischarging` is empty.
  - Tests invalid JSON, missing required fields, and invalid strategy references.
- `go build ./...` succeeds.

**Depends On:** Step 1
**Blocks:** Steps 4, 5, 6, 7

---

### Step 3: Implement Hardware Controller (ectool)

**Objective:** Implement the hardware abstraction layer and the ectool-based implementation for reading temperature, setting fan speed, checking AC status, and pausing/resuming fan control.

**Context:**
- Step 1 has established the project structure.
- This step implements the equivalent of `HardwareController.py` and `EctoolHardwareController.py`.
- This is independent of Step 2 and can be done in parallel.

**Scope:**
- Files to create:
  - `internal/hardware/hardware.go` — `HardwareController` interface
  - `internal/hardware/ectool.go` — `EctoolHardwareController` struct implementing the interface

**Sub-tasks:**
1. **Create `internal/hardware/hardware.go`:**
   - Define the `HardwareController` interface:
     ```go
     type HardwareController interface {
         GetTemperature() (float64, error)
         SetSpeed(speed int) error
         Pause() error
         Resume() error
         IsOnAC() (bool, error)
     }
     ```
   - Note: Unlike the Python version which silently swallowed errors, the Go version should return errors explicitly. The caller (fan controller) decides how to handle them.

2. **Create `internal/hardware/ectool.go`:**
   - Define `EctoolHardwareController` struct with fields:
     - `noBatterySensorMode bool`
     - `nonBatterySensors []string`
   - Implement `NewEctoolHardwareController(noBatterySensorMode bool) (*EctoolHardwareController, error)`:
     - If `noBatterySensorMode` is true, call `populateNonBatterySensors()`.
   - Implement `populateNonBatterySensors() error`:
     - Execute `ectool tempsinfo all` via `os/exec`.
     - Parse output with regex `\d+ Battery` to find battery sensor IDs.
     - Parse all sensor IDs with regex `^\d+` (multiline).
     - Store non-battery sensor IDs in `nonBatterySensors`.
     - Use `regexp.MustCompile` for the patterns.
   - Implement `GetTemperature() (float64, error)`:
     - If `noBatterySensorMode`: execute `ectool temps <id>` for each non-battery sensor, concatenate output.
     - Otherwise: execute `ectool temps all`.
     - Parse temperatures with regex `\(= (\d+) C\)`.
     - Convert to ints, filter out zeros, sort descending, return the highest.
     - **Safety fallback:** If no valid temperatures found, return `50.0` (matching Python behavior — prevents hardware damage by assuming moderate temperature).
   - Implement `SetSpeed(speed int) error`:
     - Execute `ectool fanduty <speed>`.
   - Implement `IsOnAC() (bool, error)`:
     - Execute `ectool battery`, capture stdout, suppress stderr.
     - Check for `AC_PRESENT` in the `Flags` line using regex `Flags.*(AC_PRESENT)`.
   - Implement `Pause() error`:
     - Execute `ectool autofanctrl` to restore automatic fan control.
   - Implement `Resume() error`:
     - No-op (empty implementation), matching Python behavior. Setting an arbitrary speed via `SetSpeed` implicitly disables auto fan control.

**Edge Cases & Gotchas:**
- `ectool` may not be installed or may fail. All subprocess calls should handle `exec.ErrNotFound` and non-zero exit codes gracefully, returning descriptive errors.
- The `shell=True` Python behavior means the command is passed through `/bin/sh`. In Go, use `exec.Command("ectool", "temps", "all")` directly (no shell needed), which is safer.
- Temperature parsing: the Python code does `float(round(temps[0], 2))` — the highest temperature rounded to 2 decimal places. Since ectool returns integer Celsius values, the rounding is effectively a no-op, but maintain it for compatibility.
- The `noBatterySensorMode` sensor list is populated once at startup. If sensors change (unlikely for a laptop), a restart is needed. Same as Python behavior.
- Subprocess timeout: consider adding a context with timeout (e.g., 5 seconds) to prevent hanging if ectool becomes unresponsive.

**Verification:**
- Write a test in `internal/hardware/ectool_test.go` that:
  - Tests `GetTemperature()` parsing with mock ectool output (use a helper that replaces the exec function or test the parsing logic separately).
  - Tests the safety fallback (empty output → returns 50.0).
  - Tests `IsOnAC()` parsing with mock output containing and not containing `AC_PRESENT`.
  - Tests `populateNonBatterySensors()` parsing with mock `tempsinfo` output.
- Consider extracting the parsing logic into pure functions that take string input, making them easily testable without mocking subprocess calls.

**Depends On:** Step 1
**Blocks:** Steps 4, 6

---

### Step 4: Implement Fan Controller Core Logic

**Objective:** Implement the central fan control loop — temperature reading, moving average calculation, speed curve interpolation, and the main run loop.

**Context:**
- Steps 2 and 3 have provided the configuration/strategy types and hardware controller interface.
- This step implements the equivalent of `FanController.py` — the heart of the application.

**Scope:**
- Files to create:
  - `internal/controller/controller.go` — `FanController` struct and all its methods

**Sub-tasks:**
1. **Define `FanController` struct:**
   - Fields:
     - `hardware hardware.HardwareController`
     - `config *config.Configuration`
     - `overwrittenStrategy *config.Strategy` (nil when using default)
     - `speed int`
     - `tempHistory []float64` (circular buffer, max 100 entries)
     - `active bool`
     - `timecount int`
     - `mu sync.Mutex` (protects all mutable state — **critical** since socket commands arrive on a separate goroutine)

2. **Implement `NewFanController(hw hardware.HardwareController, cfg *config.Configuration, strategyName string) *FanController`:**
   - Initialize with `active: true`, `speed: 0`, `tempHistory` as empty slice (or pre-allocated with 100 zeros matching Python's `deque([0]*100, maxlen=100)`).
   - If `strategyName` is non-empty, call `OverwriteStrategy()`.

3. **Implement temperature methods (must acquire mutex):**
   - `GetActualTemperature() (float64, error)` — delegates to hardware controller.
   - `GetMovingAverageTemperature(timeInterval int) float64`:
     - Take the last `timeInterval` entries from `tempHistory` that are > 0.
     - If empty, fall back to `GetActualTemperature()`.
     - Return the mean, rounded to 2 decimal places.
   - `GetEffectiveTemperature(currentTemp float64, timeInterval int) float64`:
     - Return `min(movingAverage, currentTemp)` rounded to 2 decimal places.
     - This matches the Python behavior exactly.

4. **Implement speed curve interpolation — `AdaptSpeed(currentTemp float64)`:**
   - Get current strategy (respecting overwrite).
   - Calculate effective temperature.
   - Walk the speed curve to find the bracketing points (min_point, max_point).
   - If same point, use that speed directly.
   - Otherwise, linear interpolation: `speed = min_speed + (temp - min_temp) * (max_speed - min_speed) / (max_temp - min_temp)`.
   - If `active`, call `SetSpeed()`.

5. **Implement strategy management (must acquire mutex):**
   - `OverwriteStrategy(name string) error` — look up strategy in config, set `overwrittenStrategy`, reset `timecount`.
   - `ClearOverwrittenStrategy()` — set `overwrittenStrategy` to nil, reset `timecount`.
   - `GetCurrentStrategy() (config.Strategy, error)` — if overwritten, return that; if on AC, return default; else return discharging.

6. **Implement pause/resume (must acquire mutex):**
   - `Pause() error` — set `active = false`, call `hardware.Pause()`.
   - `Resume() error` — set `active = true`, call `hardware.Resume()`.

7. **Implement the main run loop — `Run(debug bool) error`:**
   - Infinite loop:
     - If active: read temperature, append to history (maintain max 100 entries), every `fanSpeedUpdateFrequency` seconds call `AdaptSpeed`, increment timecount, sleep 1 second.
     - If not active: sleep 5 seconds.
   - On `InvalidStrategyError`: log error and exit (safety — don't run without a valid strategy).
   - On any other error: log error and exit (safety).
   - Use `time.Sleep` for the sleep intervals.

8. **Implement `CommandManager(args)` method** — this will be called by the socket server when a command arrives. It dispatches based on command type and returns a result. However, the actual command result types are defined in Step 5. For now, define the method signature and leave the body as a TODO, or implement it here with the result types defined inline.
   - **Better approach:** Define a `CommandHandler` interface or function type that the socket server calls. Implement the dispatch logic here in the controller, returning structured results.

**Edge Cases & Gotchas:**
- **Thread safety is critical.** The Python version relied on the GIL for implicit thread safety. In Go, the main loop and socket command handler run in separate goroutines. All reads/writes to `speed`, `active`, `overwrittenStrategy`, `tempHistory`, `timecount` must be protected by the mutex.
- The `tempHistory` circular buffer: Python uses `collections.deque(maxlen=100)`. In Go, use a slice with manual truncation: `if len(h) > 100 { h = h[len(h)-100:] }`.
- The `GetMovingAverageTemperature` filters out zeros from the history. The Python code does `[x for x in self.temp_history if x > 0][-time_interval:]` — it filters first, THEN slices. This means the time interval is in terms of non-zero readings, not total readings.
- Speed curve interpolation: the Python code iterates and finds the last point where `temp > e["temp"]` as `min_point`, and the first point where `temp <= e["temp"]` as `max_point`. Ensure the Go implementation matches this exact logic.
- The `Run` method should handle OS signals (SIGTERM, SIGINT) for graceful shutdown — restore auto fan control before exiting. This is important for systemd `ExecStopPost` but also for manual runs.

**Verification:**
- Write tests in `internal/controller/controller_test.go`:
  - Test `GetMovingAverageTemperature` with various history states (empty, all zeros, partial data, full data).
  - Test `GetEffectiveTemperature` — verify it returns `min(movingAvg, currentTemp)`.
  - Test `AdaptSpeed` with known speed curves — verify interpolation at exact points, between points, below minimum, above maximum.
  - Test strategy overwrite/clear/get logic.
  - Use a mock `HardwareController` for all tests.

**Depends On:** Steps 2, 3
**Blocks:** Steps 5, 6

---

### Step 5: Implement Command Handling and DTO Types

**Objective:** Implement the command result types (DTOs) and the command dispatch logic that translates socket commands into controller actions and formatted responses.

**Context:**
- Step 4 has implemented the fan controller core logic.
- This step implements the equivalent of all the `dto/command_result/` and `dto/runtime_result/` Python classes, plus the command dispatch logic.

**Scope:**
- Files to create:
  - `internal/dto/output.go` — `OutputFormat` enum, `Printable` interface
  - `internal/dto/command_result.go` — all command result types
  - `internal/dto/runtime_result.go` — runtime result types (status dump)
  - `internal/command/handler.go` — command dispatch logic, command types

**Sub-tasks:**
1. **Create `internal/dto/output.go`:**
   - Define `OutputFormat` as a string type with constants `Natural` and `JSON`.
   - Define `Formattable` interface with `ToOutputFormat(format OutputFormat) string` and `ToJSON() string`.

2. **Create `internal/dto/command_result.go`:**
   - Define `CommandResult` struct: `Status string` (`"success"` or `"error"`), `Reason string` (only for errors).
   - Implement `ToOutputFormat` for `CommandResult`: NATURAL returns `"Success!"` or `"[Error] > An error occurred: <reason>"`. JSON returns the struct as JSON.
   - Define all specific result types, each embedding `CommandResult`:
     - `StrategyResetCommandResult` — fields: `Strategy string`, `Default bool`. NATURAL: `"Strategy reset to default! Strategy in use: '<name>'\nDefault: <bool>"`
     - `StrategyChangeCommandResult` — fields: `Strategy string`, `Default bool`. NATURAL: `"Strategy in use: '<name>'\nDefault: <bool>"`
     - `ConfigurationReloadCommandResult` — fields: `Strategy string`, `Default bool`. NATURAL: `"Reloaded with success! Strategy in use: '<name>'\nDefault: <bool>"`
     - `ServicePauseCommandResult` — no extra fields. NATURAL: `"Service paused! The hardware fan control will take over"`
     - `ServiceResumeCommandResult` — fields: `Strategy string`, `Default bool`. NATURAL: `"Service resumed!\nStrategy in use: '<name>'\nDefault: <bool>"`
     - `PrintActiveCommandResult` — field: `Active bool`. NATURAL: `"Active: <bool>"`
     - `PrintCurrentStrategyCommandResult` — fields: `Strategy string`, `Default bool`. NATURAL: `"Strategy in use: '<name>'\nDefault: <bool>"`
     - `PrintStrategyListCommandResult` — field: `Strategies []string`. NATURAL: `"Strategy list:\n- <name1>\n- <name2>..."`
     - `PrintFanSpeedCommandResult` — field: `Speed string`. NATURAL: `"Current fan speed: '<speed>%'"`
     - `SetConfigurationCommandResult` — fields: `Strategy string`, `Configuration interface{}`, `Default bool`. NATURAL: `"Configuration updated with success: <json>.\nStrategy in use: <name>\nDefault: <bool>"`
   - Each type must implement JSON serialization that matches the Python output exactly (field names must match: `status`, `strategy`, `default`, `reason`, `strategies`, `speed`, `active`, `configuration`, `info`).

3. **Create `internal/dto/runtime_result.go`:**
   - Define `RuntimeResult` struct: `Status string`, `Reason string` (same pattern as CommandResult but for runtime output).
   - Define `StatusRuntimeResult` struct with fields: `Strategy`, `Default`, `Speed`, `Temperature`, `MovingAverageTemperature`, `EffectiveTemperature`, `Active`, `Configuration`.
   - NATURAL format must match Python exactly:
     ```
     Strategy: '<name>'
     Default: <bool>
     Speed: <speed>%
     Temp: <temp>°C
     MovingAverageTemp: <movingAvg>°C
     EffectiveTemp: <effectiveTemp>°C
     Active: <active>
     DefaultStrategy: '<defaultStrategy>'
     DischargingStrategy: '<dischargingStrategy>'
     ```
   - JSON format: serialize all fields as a JSON object with matching field names.

4. **Create `internal/command/handler.go`:**
   - Define a `CommandHandler` struct that holds a reference to the `FanController`.
   - Implement `HandleCommand(command string, args map[string]string, outputFormat dto.OutputFormat) (string, error)`:
     - Parse the command string (or accept pre-parsed command + args).
     - Dispatch to the appropriate controller method.
     - Format the result using the specified output format.
     - Return the formatted string to send back over the socket.
   - Commands to handle: `use`, `reset`, `reload`, `pause`, `resume`, `print` (with sub-selections: `all`, `active`, `current`, `list`, `speed`), `set_config`.

**Edge Cases & Gotchas:**
- The Python `True`/`False` boolean output differs from Go's `true`/`false`. The NATURAL format in Python prints `True`/`False` (capitalized). Ensure the Go output matches: Python's `print(True)` outputs `True` with capital T. The Go equivalent needs to match this for client compatibility.
- The JSON output must use the exact same field names as the Python `__dict__` serialization. In Python, `self.status = CommandStatus.SUCCESS` serializes as `"status": "success"`. Ensure Go JSON tags match.
- The `info` field in JSON output (used for legacy parser warnings) can be omitted since we're dropping legacy support.
- The `SetConfigurationCommandResult` includes the full configuration as a nested object. In Python, `vars(self.configuration)` returns `{"path": "...", "data": {...}}`. The Go version should match this structure.

**Verification:**
- Write tests that verify each command result type produces the exact expected NATURAL and JSON output strings.
- Compare output strings against the Python implementation's expected output.

**Depends On:** Step 4
**Blocks:** Steps 6, 7

---

### Step 6: Implement Daemon Binary (`fw-fanctrld`)

**Objective:** Implement the daemon's main entrypoint — CLI flag parsing, initialization of all components, Unix socket server, signal handling, and the main run loop.

**Context:**
- Steps 2-5 have implemented all the internal packages.
- This step wires everything together into the daemon binary.

**Scope:**
- Files to create/modify:
  - `cmd/fw-fanctrld/main.go` — daemon entrypoint
  - `internal/socket/server.go` — Unix socket server implementation

**Sub-tasks:**
1. **Create `internal/socket/server.go`:**
   - Define `Server` struct with fields: `socketPath string`, `listener net.Listener`, `commandHandler func(rawCommand string) (string, error)`.
   - Implement `NewServer(socketPath string, handler func(string) (string, error)) *Server`.
   - Implement `Start() error`:
     - Create parent directory (`/run/fw-fanctrl/`) if it doesn't exist.
     - Remove existing socket file if present.
     - Create `net.Listen("unix", socketPath)`.
     - Set socket file permissions to `0777` (matching Python: `os.chmod(COMMANDS_SOCKET_FILE_PATH, 0o777)`).
     - Enter accept loop:
       - Accept connection.
       - Read up to 4096 bytes from client.
       - Call `commandHandler` with the received string.
       - Send response back.
       - Shutdown write side and close connection.
     - Handle errors per-connection (don't crash the server on a single bad client).
   - Implement `Stop()` — close the listener, remove the socket file.
   - The command handler function should:
     - Parse the received command string using cobra or a simpler parser (since the daemon receives the same CLI args that the client was invoked with).
     - Extract the output format from the args.
     - Dispatch to the controller's command manager.
     - Format the result and return it.
     - On error, return a formatted error result.

2. **Implement `cmd/fw-fanctrld/main.go`:**
   - Use `cobra` for the daemon's own CLI flags:
     - `--config, -c` — config file path (default: `/etc/fw-fanctrl/config.json`)
     - `--silent, -s` — disable debug output
     - `--hardware-controller, --hc` — hardware controller type (default: `ectool`, only option for now)
     - `--socket-controller, --sc` — socket controller type (default: `unix`, only option for now)
     - `--no-battery-sensors` — disable battery temperature sensors
     - `--output-format` — output format for runtime status (default: `JSON`, matching the systemd service file)
     - `<strategy>` — optional positional arg for initial strategy override
   - Initialization sequence:
     1. Parse CLI flags.
     2. Create `EctoolHardwareController` with `noBatterySensorMode` flag.
     3. Create `Configuration` with config path.
     4. Create `FanController` with hardware controller, configuration, and optional strategy name.
     5. Create socket `Server` with command handler wired to the fan controller.
     6. Start socket server in a goroutine.
     7. Set up signal handling (SIGTERM, SIGINT) — on signal, call `hardware.Pause()` (restore auto fan control), stop socket server, exit cleanly.
     8. Call `fanController.Run(debug: !silent)` on the main goroutine (this blocks).
   - On fatal error during initialization, print error to stderr and exit with code 1.

**Edge Cases & Gotchas:**
- **Signal handling is critical for safety.** If the daemon is killed without restoring auto fan control, the fan stays at whatever speed was last set. The systemd service has `ExecStopPost=/bin/sh -c "ectool autofanctrl"` as a safety net, but the daemon should also handle SIGTERM gracefully.
- The socket server must handle concurrent connections safely. The Python version processes one connection at a time (single-threaded accept loop). The Go version should do the same — process one command at a time to avoid race conditions. Do NOT spawn a goroutine per connection.
- Socket file permissions `0777` allow any user to send commands. This matches the Python behavior and is intentional (non-root users can run `fw-fanctrl use lazy`).
- The daemon's command parser needs to parse the same command format that the CLI client sends. The client sends the full argument string (e.g., `"--output-format JSON use lazy"`). The daemon needs to parse this. Consider using a shared command definition or a simpler parser for the daemon side.
- **Important architectural detail:** In the Python version, the daemon receives the raw CLI args string over the socket and re-parses it with `CommandParser(is_remote=True)`. The `is_remote=True` flag excludes the `run` command from the parser. The Go version should do the same — the daemon-side parser should not accept `run` as a command.

**Verification:**
- The daemon starts, creates the socket file at `/run/fw-fanctrl/.fw-fanctrl.commands.sock`.
- Sending a command via `echo "print all" | socat - UNIX-CONNECT:/run/fw-fanctrl/.fw-fanctrl.commands.sock` returns a valid response.
- Sending SIGTERM causes the daemon to restore auto fan control and exit cleanly.
- The daemon logs status output to stdout when not in silent mode.

**Depends On:** Steps 2, 3, 4, 5
**Blocks:** Step 9 (for systemd service file)

---

### Step 7: Implement CLI Client Binary (`fw-fanctrl`)

**Objective:** Implement the CLI client that sends commands to the running daemon via the Unix domain socket.

**Context:**
- Step 6 has implemented the daemon with its socket server.
- This step implements the client side — the equivalent of the non-`run` path in the Python `__main__.py`.

**Scope:**
- Files to create/modify:
  - `cmd/fw-fanctrl/main.go` — CLI client entrypoint
  - `internal/socket/client.go` — Unix socket client implementation

**Sub-tasks:**
1. **Create `internal/socket/client.go`:**
   - Implement `SendCommand(socketPath string, command string) (string, error)`:
     - Create `net.Dial("unix", socketPath)`.
     - Send the command string.
     - Read the full response (loop reading 1024-byte chunks until EOF/connection closed).
     - If response starts with `"[Error] > "`, return it as an error.
     - Return the response string.
   - Handle connection errors gracefully (daemon not running → clear error message like "fw-fanctrld is not running. Start the service with: systemctl start fw-fanctrld").

2. **Implement `cmd/fw-fanctrl/main.go` using cobra:**
   - Define root command with global flags:
     - `--socket-controller, --sc` — socket controller type (default: `unix`)
     - `--output-format` — output format (`NATURAL` or `JSON`, default: `NATURAL`)
   - Define subcommands:
     - `use <strategy>` — change current strategy
     - `reset` — reset to default strategy
     - `reload` — reload configuration file
     - `pause` — pause the service
     - `resume` — resume the service
     - `print [all|active|current|list|speed]` — print information (default: `all`)
     - `set_config <json>` — replace configuration
   - Each subcommand's `RunE` function:
     1. Reconstruct the command string from the parsed args (including `--output-format` flag).
     2. Call `socket.SendCommand()` with the reconstructed command string.
     3. Print the response to stdout.
     4. If error, print to stderr and exit with code 1.
   - The command string sent over the socket should be the full argument string as the daemon expects to parse it. For example, `fw-fanctrl --output-format JSON use lazy` sends `"--output-format JSON use lazy"` over the socket.

**Edge Cases & Gotchas:**
- The client must reconstruct the argument string in a way the daemon's parser can parse. This means the global flags (`--output-format`, `--socket-controller`) must come before the subcommand, matching the argparse convention.
- If the daemon is not running, `net.Dial` will fail with "connection refused" or "no such file or directory". Provide a user-friendly error message.
- The `set_config` command takes a JSON string as an argument. This JSON may contain spaces and special characters. Ensure proper quoting/escaping when sending over the socket. The Python version uses `shlex.join(sys.argv[1:])` to reconstruct the command string.
- Response handling: if the response starts with `[Error] >`, print to stderr and exit 1. Otherwise, print to stdout and exit 0.
- Shell completion: cobra provides built-in shell completion generation. Consider adding a `completion` subcommand for bash/zsh/fish.

**Verification:**
- With the daemon running: `fw-fanctrl print all` returns the current status.
- `fw-fanctrl use lazy` changes the strategy and returns confirmation.
- `fw-fanctrl --output-format JSON print current` returns JSON-formatted output.
- `fw-fanctrl` with no args prints help text.
- Running `fw-fanctrl` when daemon is not running prints a clear error message.

**Depends On:** Steps 5, 6
**Blocks:** Step 9

---

### Step 8: Implement Unit Tests

**Objective:** Add comprehensive unit tests for all core logic — config parsing, strategy selection, speed curve interpolation, temperature averaging, and command result formatting.

**Context:**
- Steps 2-7 have implemented all functionality.
- Some tests were mentioned in earlier steps as verification. This step consolidates and expands test coverage.
- **This step is independent of Steps 9-11 and can be done in parallel.**

**Scope:**
- Files to create:
  - `internal/config/config_test.go`
  - `internal/config/strategy_test.go`
  - `internal/hardware/ectool_test.go`
  - `internal/controller/controller_test.go`
  - `internal/dto/command_result_test.go`
  - `internal/dto/runtime_result_test.go`
  - `internal/socket/server_test.go` (optional — integration-level)

**Sub-tasks:**
1. **Config tests (`internal/config/config_test.go`):**
   - Test parsing the embedded default config succeeds.
   - Test all 7 strategies are loaded with correct parameters.
   - Test `GetDefaultStrategy()` returns "lazy".
   - Test `GetDischargingStrategy()` with empty string returns default.
   - Test `GetDischargingStrategy()` with a valid strategy name returns that strategy.
   - Test `GetStrategy()` with invalid name returns `InvalidStrategyError`.
   - Test `Parse()` with invalid JSON returns `ConfigurationParsingError`.
   - Test `Parse()` with valid JSON but missing required fields fails schema validation.
   - Test `Parse()` with `defaultStrategy` pointing to non-existent strategy fails.
   - Test `Parse()` with `strategyOnDischarging` pointing to non-existent strategy fails.
   - Test `Reload()` creates default config file when path doesn't exist (use temp directory).
   - Test `Save()` writes valid JSON that can be re-parsed.

2. **Strategy tests (`internal/config/strategy_test.go`):**
   - Test default values: `FanSpeedUpdateFrequency` defaults to 5 when 0/missing.
   - Test default values: `MovingAverageInterval` defaults to 20 when 0/missing.
   - Test speed curve is preserved correctly.

3. **Hardware controller tests (`internal/hardware/ectool_test.go`):**
   - Extract parsing logic into testable pure functions:
     - `parseTemperatures(output string) []int`
     - `parseACPresent(output string) bool`
     - `parseNonBatterySensors(output string) []string`
   - Test `parseTemperatures` with real ectool output samples.
   - Test safety fallback: empty output → returns 50.
   - Test `parseACPresent` with AC present and absent.
   - Test `parseNonBatterySensors` with mixed battery/non-battery sensors.

4. **Controller tests (`internal/controller/controller_test.go`):**
   - Create a `MockHardwareController` that implements the interface with configurable return values.
   - Test `GetMovingAverageTemperature`:
     - Empty history → falls back to actual temperature.
     - All zeros → falls back to actual temperature.
     - Normal data → correct average.
     - More data than `timeInterval` → only uses last N non-zero entries.
   - Test `GetEffectiveTemperature`:
     - Returns min of moving average and current temp.
   - Test `AdaptSpeed` with the "lazy" strategy speed curve:
     - At 0°C → 15% speed.
     - At 50°C → 15% speed.
     - At 57.5°C → 20% speed (interpolated between 50°C/15% and 65°C/25%).
     - At 65°C → 25% speed.
     - At 85°C → 100% speed.
     - At 100°C → 100% speed (above max point).
   - Test strategy overwrite and clear.
   - Test pause/resume toggles `active` flag.

5. **DTO tests (`internal/dto/command_result_test.go` and `runtime_result_test.go`):**
   - Test each result type's NATURAL output matches expected string exactly.
   - Test each result type's JSON output is valid JSON with correct field names and values.
   - Pay special attention to boolean capitalization (`True`/`False` vs `true`/`false`).

**Edge Cases & Gotchas:**
- Use `t.TempDir()` for tests that need to write config files to disk.
- The mock hardware controller should be defined in a `_test.go` file or in an `internal/hardware/mock.go` file for reuse.
- Floating point comparison in temperature tests: use a small epsilon for comparison rather than exact equality.

**Verification:**
- `go test ./...` passes with all tests green.
- `go test -cover ./...` shows reasonable coverage (aim for >80% on `internal/config/`, `internal/controller/`, `internal/hardware/` parsing logic).

**Depends On:** Steps 2, 3, 4, 5
**Blocks:** Step 10 (CI needs tests to run)

---

### Step 9: Implement Makefile and Installation Assets

**Objective:** Create the Makefile for building, installing, and uninstalling the project, plus the systemd service file and suspend hook templates.

**Context:**
- Steps 6 and 7 have produced the two binaries.
- This step creates the build/install infrastructure.

**Scope:**
- Files to create/modify:
  - `Makefile`
  - `services/fw-fanctrld.service` (new template for the Go daemon)
  - `services/system-sleep/fw-fanctrl-suspend` (updated template)

**Sub-tasks:**
1. **Create `Makefile`:**
   - Variables (overridable):
     - `PREFIX ?= /usr`
     - `DESTDIR ?=`
     - `SYSCONFDIR ?= /etc`
     - `BINDIR ?= $(PREFIX)/bin`
     - `SYSTEMDDIR ?= $(PREFIX)/lib/systemd`
     - `GO ?= go`
     - `VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")`
     - `LDFLAGS ?= -s -w -X main.version=$(VERSION)`
   - Targets:
     - `all: build` (default)
     - `build:` — build both binaries:
       - `$(GO) build -ldflags "$(LDFLAGS)" -o bin/fw-fanctrl ./cmd/fw-fanctrl`
       - `$(GO) build -ldflags "$(LDFLAGS)" -o bin/fw-fanctrld ./cmd/fw-fanctrld`
     - `test:` — `$(GO) test ./...`
     - `lint:` — `golangci-lint run ./...`
     - `install:` — install binaries, config, systemd service, suspend hook:
       - Install binaries to `$(DESTDIR)$(BINDIR)/`
       - Install default config to `$(DESTDIR)$(SYSCONFDIR)/fw-fanctrl/config.json` (don't overwrite existing)
       - Install schema to `$(DESTDIR)$(SYSCONFDIR)/fw-fanctrl/config.schema.json`
       - Generate and install systemd service file (sed template substitution for binary path and config path)
       - Install suspend hook script
     - `install-ectool:` — download and install ectool (same logic as current `installEctool` bash function)
     - `uninstall:` — remove installed files, stop/disable service
     - `enable:` — `systemctl daemon-reload && systemctl enable --now fw-fanctrld`
     - `disable:` — `systemctl stop fw-fanctrld && systemctl disable fw-fanctrld`
     - `clean:` — `rm -rf bin/`
     - `.PHONY: all build test lint install uninstall enable disable clean install-ectool`

2. **Create `services/fw-fanctrld.service`:**
   ```ini
   [Unit]
   Description=Framework Fan Controller Daemon
   After=multi-user.target

   [Service]
   Type=simple
   Restart=always
   ExecStart=%BINDIR%/fw-fanctrld --output-format JSON --config %SYSCONFDIR%/fw-fanctrl/config.json --silent %NO_BATTERY_SENSOR_OPTION%
   ExecStopPost=/bin/sh -c "ectool autofanctrl"

   [Install]
   WantedBy=multi-user.target
   ```
   - Note the service name changes from `fw-fanctrl.service` to `fw-fanctrld.service` to match the daemon binary name.

3. **Update `services/system-sleep/fw-fanctrl-suspend`:**
   ```sh
   #!/bin/sh
   case $1 in
       pre)  %BINDIR%/fw-fanctrl pause ;;
       post) %BINDIR%/fw-fanctrl resume ;;
   esac
   ```

4. **Handle the `--no-battery-sensors` flag in the Makefile:**
   - Add a `NO_BATTERY_SENSORS ?=` variable. If set to `1`, add `--no-battery-sensors` to the service file during install.

**Edge Cases & Gotchas:**
- The `install` target should use `install -Dm755` for binaries and `install -Dm644` for config files, following Linux packaging conventions.
- Config file installation should NOT overwrite an existing config (`install` with `-n` flag or a conditional check). The schema file CAN be overwritten (it's not user-modified).
- The Makefile must handle the `DESTDIR` variable correctly for package builders (e.g., Arch Linux PKGBUILD, RPM spec files).
- The ectool installation target should verify the SHA256 hash, matching the current `install.sh` behavior.

**Verification:**
- `make build` produces two binaries in `bin/`.
- `make test` runs all tests.
- `sudo make install` installs everything to the correct locations.
- `sudo make enable` starts the service.
- `sudo make uninstall` cleanly removes everything.
- `make clean` removes build artifacts.

**Depends On:** Steps 6, 7
**Blocks:** Step 10

---

### Step 10: Implement CI/CD Workflows

**Objective:** Create GitHub Actions workflows for automated testing, linting, and cross-compiled release builds.

**Context:**
- Steps 8 and 9 have established tests and the build system.
- This step adds automation.

**Scope:**
- Files to create:
  - `.github/workflows/test.yml` — run tests and linting on PRs and pushes
  - `.github/workflows/release.yml` — build and publish release binaries on tag push
  - `.golangci.yml` — linter configuration

**Sub-tasks:**
1. **Create `.github/workflows/test.yml`:**
   - Trigger: push to `main`, pull requests to `main`.
   - Jobs:
     - `test`: Run on `ubuntu-latest`.
       - Checkout code.
       - Set up Go (version 1.21+).
       - Run `go test ./...`.
       - Run `go vet ./...`.
     - `lint`: Run on `ubuntu-latest`.
       - Checkout code.
       - Set up Go.
       - Install and run `golangci-lint` (use the official GitHub Action `golangci/golangci-lint-action`).

2. **Create `.github/workflows/release.yml`:**
   - Trigger: push of tags matching `v*`.
   - Job: `release` on `ubuntu-latest`.
     - Checkout code.
     - Set up Go.
     - Build for multiple platforms:
       - `linux/amd64`
       - `linux/arm64`
     - For each platform, build both binaries and create a tarball:
       - `fw-fanctrl-<version>-linux-<arch>.tar.gz` containing `fw-fanctrl`, `fw-fanctrld`, `config.json`, `config.schema.json`.
     - Create GitHub Release using `softprops/action-gh-release` with the tarballs attached.
   - Cross-compilation in Go: set `GOOS=linux GOARCH=amd64` and `GOOS=linux GOARCH=arm64` environment variables.

3. **Create `.golangci.yml`** (linter configuration):
   - Enable useful linters: `errcheck`, `govet`, `staticcheck`, `unused`, `ineffassign`, `gosimple`.
   - Set reasonable timeouts and exclusions.

**Edge Cases & Gotchas:**
- The release workflow should extract the version from the git tag (strip the `v` prefix).
- Cross-compiled binaries should be statically linked (`CGO_ENABLED=0`) to avoid glibc version issues.
- The test workflow should cache Go modules for faster CI runs.

**Verification:**
- Push a commit to a PR branch → test and lint workflows run and pass.
- Push a tag `v1.0.0` → release workflow builds binaries for both architectures and creates a GitHub release.

**Depends On:** Steps 8, 9
**Blocks:** None

---

### Step 11: Update Documentation

**Objective:** Write the README and documentation for the new Go-based project.

**Context:**
- All code and infrastructure is complete.
- This step creates user-facing documentation.

**Scope:**
- Files to create/modify:
  - `README.md` — project overview, installation instructions, usage
  - `doc/commands.md` — command reference (can be largely reused from original)
  - `doc/configuration.md` — configuration guide (can be largely reused from original)

**Sub-tasks:**
1. **Write `README.md`:**
   - Project description (same as original but mention it's written in Go, no Python dependency).
   - Requirements: Linux kernel >= 6.11, Go >= 1.21 (only for building from source), ectool.
   - Installation instructions:
     - From release: download tarball, extract, `sudo make install && sudo make enable`.
     - From source: `git clone`, `make build`, `sudo make install && sudo make enable`.
   - Update instructions: download new release, `sudo make install`.
   - Uninstall instructions: `sudo make disable && sudo make uninstall`.
   - Quick usage examples.
   - Link to detailed docs.
   - Third-party projects section (same as original).
   - Development setup: `go test ./...`, `golangci-lint run`.
   - Migration note from Python version (mention config file is compatible, but service name changed from `fw-fanctrl` to `fw-fanctrld`, and legacy CLI flags are no longer supported).

2. **Update `doc/commands.md`:**
   - Remove legacy command references.
   - Update binary names (`fw-fanctrl` for client, `fw-fanctrld` for daemon).
   - Remove the `run` command from the client docs (it's now a daemon-only command).
   - Keep all other commands the same.

3. **Update `doc/configuration.md`:**
   - Largely unchanged — the config format is identical.
   - Update any Python-specific references.

**Edge Cases & Gotchas:**
- The service name change from `fw-fanctrl.service` to `fw-fanctrld.service` is a breaking change for users upgrading. Document the migration path clearly: stop old service, uninstall old version, install new version.
- Third-party projects (fw-fanctrl-gui, GNOME extension, etc.) communicate via the Unix socket. As long as the socket path and protocol are unchanged, they should continue to work. Document this compatibility.

**Verification:**
- README renders correctly on GitHub.
- Installation instructions can be followed from scratch on a clean system.
- All links in documentation are valid.

**Depends On:** Steps 6, 7, 9
**Blocks:** None

**This step is independent of Steps 8 and 10 and can be executed in parallel with them.**
