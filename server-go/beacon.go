package main

import "encoding/json"

const (
	beaconPort       = 8000
	beaconAddr       = "255.255.255.255"
	beaconIntervalMs = 2000
)

func beaconMessage(name string, port int) string {
	b, _ := json.Marshal(map[string]any{"app": "cardputerme", "v": 1, "name": name, "port": port})
	return string(b)
}
