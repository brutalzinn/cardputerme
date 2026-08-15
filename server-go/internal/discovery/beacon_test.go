package discovery

import "testing"

func TestBeaconMessage(t *testing.T) {
	got := BeaconMessage("myproj", 8003)
	want := `{"app":"cardputerme","name":"myproj","port":8003,"v":1}`
	if got != want {
		t.Fatalf("BeaconMessage = %s, want %s", got, want)
	}
}

func TestBeaconConstants(t *testing.T) {
	if BeaconPort != 8000 || BeaconAddr != "255.255.255.255" || BeaconIntervalMs != 2000 {
		t.Fatalf("beacon constants drifted: %d %s %d", BeaconPort, BeaconAddr, BeaconIntervalMs)
	}
}
