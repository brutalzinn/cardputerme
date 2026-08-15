package discovery

import "encoding/json"

const (
	BeaconPort       = 8000
	BeaconAddr       = "255.255.255.255"
	BeaconIntervalMs = 2000
)

func BeaconMessage(name string, port int) string {
	b, _ := json.Marshal(map[string]any{"app": "cardputerme", "v": 1, "name": name, "port": port})
	return string(b)
}
