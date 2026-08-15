SERVER_DIR := server-go
FW_DIR := firmware/cardputer
DIST := dist
VERSION ?= dev
PLATFORMS := darwin/arm64 darwin/amd64 linux/amd64 linux/arm64
PIO := $(shell command -v pio 2>/dev/null || echo $(HOME)/.platformio/penv/bin/pio)
SHA256 := $(shell command -v sha256sum 2>/dev/null || echo "shasum -a 256")

.DEFAULT_GOAL := help
.PHONY: help setup test build flash release release-checksums release-upload release-publish clean

help:
	@echo "make setup             install Go deps + create firmware/.env"
	@echo "make test              run the server tests"
	@echo "make build             build the server binary"
	@echo "make flash             upload firmware to the Cardputer"
	@echo "make release           build CLI binaries into dist/ (override PLATFORMS=os/arch ...)"
	@echo "make release-upload VERSION=vX.Y.Z    checksum dist/ and publish it to a GitHub Release"
	@echo "make release-publish VERSION=vX.Y.Z   build every platform, then publish"
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
	cp bin/cardputer-server $(DIST)/cardputerme
	@echo "release binaries in $(DIST)/"

release-checksums:
	cd $(DIST) && rm -f checksums.txt && $(SHA256) cardputerme* > checksums.txt

release-upload: release-checksums
	gh release create $(VERSION) $(DIST)/* --title "cardputerme $(VERSION)" \
		--notes "Install: \`curl -fsSL https://raw.githubusercontent.com/brutalzinn/cardputerme/main/install.sh | sh\`

Native builds: macOS Intel + Apple Silicon, Linux amd64 + arm64. Static (CGO off) — runs on any Ubuntu/glibc version. Requires \`tmux\` at runtime."

release-publish: release release-upload

clean:
	rm -f $(SERVER_DIR)/cardputerme
	rm -rf $(DIST)
