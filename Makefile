SERVER_DIR := server-go
FW_DIR := firmware/cardputer
FW_ENV := cardputer-adv
PIO := $(shell command -v pio 2>/dev/null || echo $(HOME)/.platformio/penv/bin/pio)
DEADCODE := go run golang.org/x/tools/cmd/deadcode@latest
NAME ?= $(notdir $(CURDIR))
CLAUDE_DIRS := $(wildcard $(HOME)/.claude $(HOME)/.claude-account1 $(HOME)/.claude-account2)

.DEFAULT_GOAL := help
.PHONY: help setup deps env build test test-race vet deadcode check \
        run expose flash upload monitor skill-install skill-uninstall clean

help:
	@echo "cardputerme — expose any terminal to an M5Cardputer"
	@echo ""
	@echo "Setup"
	@echo "  make setup           one-shot: go deps + firmware .env + /cardputer skill"
	@echo "  make deps            fetch Go modules, verify pio is installed"
	@echo "  make env             create firmware/.env from the template (edit Wi-Fi after)"
	@echo ""
	@echo "Server (Go)"
	@echo "  make build           build the server binary (server-go/cardputerme)"
	@echo "  make test            go test ./..."
	@echo "  make test-race       go test -race ./..."
	@echo "  make vet             go vet ./..."
	@echo "  make deadcode        report unreachable code"
	@echo "  make check           vet + test-race + deadcode (pre-commit gate)"
	@echo ""
	@echo "Run / device"
	@echo "  make expose [NAME=x] start a background server exposing this terminal"
	@echo "  make flash           build + upload firmware to the Cardputer"
	@echo "  make monitor         open the firmware serial monitor"
	@echo ""
	@echo "Maintenance"
	@echo "  make skill-install   symlink /cardputer into your Claude account(s)"
	@echo "  make clean           remove the built server binary"

setup: deps env skill-install
	@echo ""
	@echo "Ready. Next:"
	@echo "  1. edit $(FW_DIR)/.env  (set WIFI_SSID + WIFI_PASS)"
	@echo "  2. make flash          (upload firmware to the Cardputer)"
	@echo "  3. make expose         (expose this terminal) then pick it on the device"

deps:
	@command -v go >/dev/null || { echo "Go not found — install Go >= 1.22"; exit 1; }
	cd $(SERVER_DIR) && go mod download
	@test -x "$(PIO)" || command -v pio >/dev/null || \
		echo "note: PlatformIO (pio) not found — needed only for 'make flash'. Install: https://platformio.org/install/cli"

env:
	@test -f $(FW_DIR)/.env && echo "$(FW_DIR)/.env already exists — leaving it" || \
		{ cp $(FW_DIR)/.env.example $(FW_DIR)/.env && echo "created $(FW_DIR)/.env — set WIFI_SSID and WIFI_PASS"; }

build:
	cd $(SERVER_DIR) && go build -o cardputerme ./cmd/cardputerme

test:
	cd $(SERVER_DIR) && go test ./...

test-race:
	cd $(SERVER_DIR) && go test -race ./...

vet:
	cd $(SERVER_DIR) && go vet ./...

deadcode:
	cd $(SERVER_DIR) && $(DEADCODE) -test ./...

check: vet test-race deadcode

run: expose
expose:
	./bin/cardputer-server "$(NAME)"

flash upload:
	cd $(FW_DIR) && "$(PIO)" run -e $(FW_ENV) -t upload

monitor:
	cd $(FW_DIR) && "$(PIO)" device monitor

skill-install:
	@for d in $(CLAUDE_DIRS); do \
		mkdir -p "$$d/skills"; \
		ln -sfn "$(CURDIR)/skills/cardputer" "$$d/skills/cardputer"; \
		echo "linked $$d/skills/cardputer"; \
	done
	@test -n "$(CLAUDE_DIRS)" || echo "no ~/.claude* directory found — skipping skill install"

skill-uninstall:
	@for d in $(CLAUDE_DIRS); do \
		rm -f "$$d/skills/cardputer" && echo "removed $$d/skills/cardputer"; \
	done

clean:
	rm -f $(SERVER_DIR)/cardputerme
