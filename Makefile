SERVER_DIR := server-go
FW_DIR := firmware/cardputer
DIST := dist
VERSION ?= dev
PLATFORMS := darwin/arm64 darwin/amd64 linux/amd64 linux/arm64
PIO := $(shell command -v pio 2>/dev/null || echo $(HOME)/.platformio/penv/bin/pio)

.DEFAULT_GOAL := help
.PHONY: help setup test build flash release release-publish clean

help:
	@echo "make setup             install Go deps + create firmware/.env"
	@echo "make test              run the server tests"
	@echo "make build             build the server binary"
	@echo "make flash             upload firmware to the Cardputer"
	@echo "make release           cross-compile CLI binaries into dist/ (macOS + Linux)"
	@echo "make release-publish VERSION=vX.Y.Z   publish dist/ to a GitHub Release"
	@echo "make clean             remove built binaries + dist/"

setup:
	cd $(SERVER_DIR) && go mod download
	@test -f $(FW_DIR)/.env || cp $(FW_DIR)/.env.example $(FW_DIR)/.env
	@echo "setup done — edit $(FW_DIR)/.env (WIFI_SSID + WIFI_PASS), then: make flash"

test:
	cd $(SERVER_DIR) && go test ./...

build:
	cd $(SERVER_DIR) && go build -o cardputerme ./cmd/cardputerme

flash:
	cd $(FW_DIR) && "$(PIO)" run -e cardputer-adv -t upload

release:
	rm -rf $(DIST) && mkdir -p $(DIST)
	@for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; out=$(CURDIR)/$(DIST)/cardputerme-$$os-$$arch; \
		echo "building $$out"; \
		( cd $(SERVER_DIR) && CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -ldflags "-s -w" -o "$$out" ./cmd/cardputerme ) || exit 1; \
	done
	@echo "release binaries in $(DIST)/"

release-publish: release
	gh release create $(VERSION) $(DIST)/* --title "cardputerme $(VERSION)" --notes "Prebuilt cardputerme CLI binaries for macOS and Linux (amd64 + arm64)."

clean:
	rm -f $(SERVER_DIR)/cardputerme
	rm -rf $(DIST)
