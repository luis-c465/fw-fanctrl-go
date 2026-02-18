PREFIX ?= /usr
DESTDIR ?=
SYSCONFDIR ?= /etc
BINDIR ?= $(PREFIX)/bin
SYSTEMDDIR ?= $(PREFIX)/lib/systemd
GO ?= go
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || printf "dev")
LDFLAGS ?= -s -w -X main.version=$(VERSION)
NO_BATTERY_SENSORS ?= 0

BIN_DIR := bin
CLIENT_BIN := $(BIN_DIR)/fw-fanctrl
DAEMON_BIN := $(BIN_DIR)/fw-fanctrld

CONFIG_DIR := $(DESTDIR)$(SYSCONFDIR)/fw-fanctrl
SYSTEMD_SYSTEM_DIR := $(DESTDIR)$(SYSTEMDDIR)/system
SYSTEMD_SLEEP_DIR := $(DESTDIR)$(SYSTEMDDIR)/system-sleep

SERVICE_TEMPLATE := services/fw-fanctrld.service
SUSPEND_TEMPLATE := services/system-sleep/fw-fanctrl-suspend

SERVICE_FILE := $(SYSTEMD_SYSTEM_DIR)/fw-fanctrld.service
SUSPEND_FILE := $(SYSTEMD_SLEEP_DIR)/fw-fanctrl-suspend

NO_BATTERY_SENSOR_OPTION :=
ifeq ($(NO_BATTERY_SENSORS),1)
NO_BATTERY_SENSOR_OPTION := --no-battery-sensors
endif

all: build

build:
	mkdir -p "$(BIN_DIR)"
	$(GO) build -ldflags "$(LDFLAGS)" -o "$(CLIENT_BIN)" ./cmd/fw-fanctrl
	$(GO) build -ldflags "$(LDFLAGS)" -o "$(DAEMON_BIN)" ./cmd/fw-fanctrld

test:
	$(GO) test ./...

lint:
	golangci-lint run ./...

install: build
	install -Dm755 "$(CLIENT_BIN)" "$(DESTDIR)$(BINDIR)/fw-fanctrl"
	install -Dm755 "$(DAEMON_BIN)" "$(DESTDIR)$(BINDIR)/fw-fanctrld"
	mkdir -p "$(CONFIG_DIR)"
	if [ ! -f "$(CONFIG_DIR)/config.json" ]; then install -Dm644 resources/config.json "$(CONFIG_DIR)/config.json"; fi
	install -Dm644 resources/config.schema.json "$(CONFIG_DIR)/config.schema.json"
	mkdir -p "$(SYSTEMD_SYSTEM_DIR)"
	sed -e "s|%BINDIR%|$(BINDIR)|g" -e "s|%SYSCONFDIR%|$(SYSCONFDIR)|g" -e "s|%NO_BATTERY_SENSOR_OPTION%|$(NO_BATTERY_SENSOR_OPTION)|g" "$(SERVICE_TEMPLATE)" > "$(SERVICE_FILE)"
	chmod 644 "$(SERVICE_FILE)"
	mkdir -p "$(SYSTEMD_SLEEP_DIR)"
	sed -e "s|%BINDIR%|$(BINDIR)|g" "$(SUSPEND_TEMPLATE)" > "$(SUSPEND_FILE)"
	chmod 755 "$(SUSPEND_FILE)"

uninstall:
	if [ -z "$(DESTDIR)" ]; then $(MAKE) disable || true; fi
	rm -f "$(DESTDIR)$(BINDIR)/fw-fanctrl"
	rm -f "$(DESTDIR)$(BINDIR)/fw-fanctrld"
	rm -f "$(SERVICE_FILE)"
	rm -f "$(SUSPEND_FILE)"
	rm -f "$(CONFIG_DIR)/config.schema.json"
	rm -f "$(CONFIG_DIR)/config.json"
	rmdir --ignore-fail-on-non-empty "$(CONFIG_DIR)" 2>/dev/null || true

enable:
	systemctl daemon-reload && systemctl enable --now fw-fanctrld

disable:
	systemctl stop fw-fanctrld && systemctl disable fw-fanctrld

clean:
	rm -rf "$(BIN_DIR)"

.PHONY: all build test lint install uninstall enable disable clean
