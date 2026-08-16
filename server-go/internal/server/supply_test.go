package server

import (
	"testing"
	"time"

	"cardputerme/internal/power"
)

func TestAConnectedUsbHostMeansExternalPower(t *testing.T) {
	if !onExternalPower(true, 3600, 4200) {
		t.Fatal("a USB host is plugged in, whatever the battery reads")
	}
}

func TestAChargingVoltageMeansExternalPower(t *testing.T) {
	if !onExternalPower(false, 4250, 4200) {
		t.Fatal("above the threshold the device is being fed")
	}
}

func TestABatteryOnlyDeviceIsNotExternallyPowered(t *testing.T) {
	if onExternalPower(false, 3900, 4200) {
		t.Fatal("a discharging battery must still let the screen sleep")
	}
}

func TestTheThresholdIsTheServersToChoose(t *testing.T) {
	if onExternalPower(false, 4100, 4200) {
		t.Fatal("below the configured threshold")
	}
	if !onExternalPower(false, 4100, 4000) {
		t.Fatal("a lower configured threshold changes the verdict without re-flashing")
	}
}

func TestAReportOnUsbStopsTheScreenSleeping(t *testing.T) {
	s := sleepyServer()
	s.handleReport(true, 4300, time.Now())
	if got := s.power.Until(time.Now()); got != 0 {
		t.Fatalf("no rung should be armed while on USB, got %v", got)
	}
}

func TestUnpluggingLetsItSleepAgain(t *testing.T) {
	s := sleepyServer()
	now := time.Now()
	s.handleReport(true, 4300, now)
	s.handleReport(false, 3800, now)
	if got := s.power.Until(now); got <= 0 {
		t.Fatalf("the ladder must re-arm once unplugged, got %v", got)
	}
	if st, _ := s.power.At(now.Add(45 * time.Second)); st != power.Dim {
		t.Fatalf("got %q", st)
	}
}
