package main

import "testing"

func TestBeaconMessage(t *testing.T) {
	got := beaconMessage("myproj", 8003)
	want := `{"app":"cardputerme","name":"myproj","port":8003,"v":1}`
	if got != want {
		t.Fatalf("beaconMessage = %s, want %s", got, want)
	}
}

func TestBeaconConstants(t *testing.T) {
	if beaconPort != 8000 || beaconAddr != "255.255.255.255" || beaconIntervalMs != 2000 {
		t.Fatalf("beacon constants drifted: %d %s %d", beaconPort, beaconAddr, beaconIntervalMs)
	}
}
