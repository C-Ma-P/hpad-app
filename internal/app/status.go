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
		Detail: "Waiting for a Desktop Dongle or Desktop BLE report",
	}
}

func disconnectedMacropadStatus() DeviceConnectionStatus {
	return disconnectedMacropadStatusForSource(device.SourceUSB)
}

func disconnectedMacropadStatusForSource(source device.Source) DeviceConnectionStatus {
	detail := "USB receiver is online, waiting for the wireless macropad"
	if source == device.SourceBLE {
		detail = "Desktop BLE link is disconnected"
	}
	return DeviceConnectionStatus{
		State:  "disconnected",
		Label:  "Disconnected",
		Detail: detail,
	}
}

func connectedMacropadStatus() DeviceConnectionStatus {
	return connectedMacropadStatusForSource(device.SourceUSB)
}

func connectedMacropadStatusForSource(source device.Source) DeviceConnectionStatus {
	detail := "Wireless macropad is reporting through Desktop Dongle"
	if source == device.SourceBLE {
		detail = "Wireless macropad is reporting through Desktop BLE"
	}
	return DeviceConnectionStatus{
		State:  "connected",
		Label:  "Connected",
		Detail: detail,
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
