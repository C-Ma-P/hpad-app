package app

import (
	"fmt"

	"hpad-app/internal/device"
)

func disconnectedDongleStatus(reason string) DeviceConnectionStatus {
	detail := "USB HID dongle not detected"
	if reason != "" {
		detail = reason
	}
	return DeviceConnectionStatus{
		State:  "not_detected",
		Label:  "Not Detected",
		Detail: detail,
	}
}

func unknownMacropadStatus() DeviceConnectionStatus {
	return DeviceConnectionStatus{
		State:  "unknown",
		Label:  "Unknown",
		Detail: "Waiting for dongle",
	}
}

func disconnectedMacropadStatus() DeviceConnectionStatus {
	return DeviceConnectionStatus{
		State:  "disconnected",
		Label:  "Disconnected",
		Detail: "Dongle is online, waiting for the wireless macropad",
	}
}

func connectedMacropadStatus() DeviceConnectionStatus {
	return DeviceConnectionStatus{
		State:  "connected",
		Label:  "Connected",
		Detail: "Wireless macropad is reporting through the dongle",
	}
}

func waitingBatteryStatus() BatteryStatus {
	return BatteryStatus{
		State:           "waiting",
		Label:           "--.- V",
		Detail:          "Waiting for device report",
		BatteryMV:       0,
		USBPowerPresent: false,
	}
}

func batteryStatusFromReport(report device.Report) BatteryStatus {
	if report.BatteryMV == 0 {
		return waitingBatteryStatus()
	}

	detail := "Running on battery power"
	state := "connected"
	if report.USBPowerPresent {
		detail = "USB power present"
		state = "usb_power"
	}

	return BatteryStatus{
		State:           state,
		Label:           formatBatteryMV(report.BatteryMV),
		Detail:          detail,
		BatteryMV:       int(report.BatteryMV),
		USBPowerPresent: report.USBPowerPresent,
	}
}

func formatBatteryMV(batteryMV uint16) string {
	whole := batteryMV / 1000
	fraction := (batteryMV % 1000) / 10
	return fmt.Sprintf("%d.%02d V", whole, fraction)
}
