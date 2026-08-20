SERVER_DIR := server-go
FW_DIR := firmware/cardputer
DIST := dist
VERSION ?= dev
PLATFORMS := darwin/arm64 darwin/amd64 linux/amd64 linux/arm64
PIO := $(shell command -v pio 2>/dev/null || echo $(HOME)/.platformio/penv/bin/pio)
SHA256 := $(shell command -v sha256sum 2>/dev/null || echo "shasum -a 256")

define NEWLINE


endef

RELEASE_NOTES := Install: curl -fsSL https://raw.githubusercontent.com/brutalzinn/cardputerme/main/install.sh | sh$(NEWLINE)$(NEWLINE)Native builds — macOS (Intel + Apple Silicon) and Linux (amd64 + arm64), static (CGO off) so any Ubuntu version works. Requires tmux at runtime.

.DEFAULT_GOAL := help
.PHONY: help setup test build run watch flash cardputer release release-checksums release-upload release-publish clean

help:
	@echo "quickstart              make setup && make run"
	@echo ""
	@echo "make setup             check deps (go, tmux) + install Go modules + create firmware/.env"
	@echo "make run               run the server straight from source, no build step (NAME=foo to label it)"
	@echo "make watch             like run, but restarts on every *.go save (needs entr: brew/apt install entr)"
	@echo "make test              run the server tests"
	@echo "make build             build the server binary into $(SERVER_DIR)/cardputerme"
	@echo "make flash             upload firmware to the Cardputer"
	@echo "make cardputer         link this terminal to the cardputerme server (uses the built/prebuilt binary)"
	@echo "make release           build CLI binaries into dist/ (override PLATFORMS=os/arch ...)"
	@echo "make release-upload VERSION=vX.Y.Z    checksum dist/ and publish it to a GitHub Release"
	@echo "make release-publish VERSION=vX.Y.Z   build every platform, then publish"
	@echo "make clean             remove built binaries + dist/"

setup:
	@command -v go >/dev/null 2>&1 || { echo "go is required — https://go.dev/dl (>= 1.26)"; exit 1; }
	@command -v tmux >/dev/null 2>&1 || { echo "tmux is required — brew install tmux (macOS) / sudo apt install tmux (Linux)"; exit 1; }
	cd $(SERVER_DIR) && go mod download
	@test -f $(FW_DIR)/.env || cp $(FW_DIR)/.env.example $(FW_DIR)/.env
	@echo "setup done — next: make run"

test:
	cd $(SERVER_DIR) && go test ./...

build:
	cd $(SERVER_DIR) && go build -o cardputerme ./cmd/cardputerme

run:
	cd $(SERVER_DIR) && go run ./cmd/cardputerme $(NAME)

watch:
	@command -v entr >/dev/null 2>&1 || { echo "entr is required — brew install entr (macOS) / sudo apt install entr (Linux)"; exit 1; }
	cd $(SERVER_DIR) && find . -name '*.go' | entr -r -- go run ./cmd/cardputerme $(NAME)

flash:
	cd $(FW_DIR) && "$(PIO)" run -e cardputer-adv -t upload

cardputer:
	./bin/cardputer-server

release:
	rm -rf $(DIST) && mkdir -p $(DIST)
	@for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; out=$(CURDIR)/$(DIST)/cardputerme-$$os-$$arch; \
		echo "building $$out"; \
		( cd $(SERVER_DIR) && CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -ldflags "-s -w" -o "$$out" ./cmd/cardputerme ) || exit 1; \
	done
	cp bin/cardputer-server $(DIST)/cardputerme
	@echo "release binaries in $(DIST)/"

release-checksums:
	cd $(DIST) && rm -f checksums.txt && $(SHA256) cardputerme* > checksums.txt

release-upload: release-checksums
	@if gh release view $(VERSION) >/dev/null 2>&1; then \
		echo "release $(VERSION) already exists — replacing its assets"; \
		gh release upload $(VERSION) $(DIST)/* --clobber; \
	else \
		gh release create $(VERSION) $(DIST)/* --title "cardputerme $(VERSION)" --notes "$(RELEASE_NOTES)"; \
	fi

release-publish: release release-upload

clean:
	rm -f $(SERVER_DIR)/cardputerme
	rm -rf $(DIST)
