SERVER_DIR := server-go
FW_DIR := firmware/cardputer
PIO := $(shell command -v pio 2>/dev/null || echo $(HOME)/.platformio/penv/bin/pio)

.DEFAULT_GOAL := help
.PHONY: help setup test build flash clean

help:
	@echo "make setup   install Go deps + create firmware/.env"
	@echo "make test    run the server tests"
	@echo "make build   build the server binary"
	@echo "make flash   upload firmware to the Cardputer"
	@echo "make clean   remove the built binary"

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

clean:
	rm -f $(SERVER_DIR)/cardputerme
