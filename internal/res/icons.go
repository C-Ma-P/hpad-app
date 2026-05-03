package res

import _ "embed"

var (
	//go:embed hpad-icon.png
	AppIcon []byte

	//go:embed hpad-connected.png
	TrayConnectedIcon []byte

	//go:embed hpad-disconnected.png
	TrayDisconnectedIcon []byte

	//go:embed hpad-charging.png
	TrayUSBPowerIcon []byte

	//go:embed hpad-low-battery.png
	TrayLowBatteryIcon []byte

	//go:embed hpad-error.png
	TrayErrorIcon []byte
)
