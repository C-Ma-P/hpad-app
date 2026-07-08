package device

const (
	vendorID          uint16 = 0xCAFE
	productID         uint16 = 0xB00B
	vendorUsagePage   uint16 = 0xFF00
	vendorUsage       uint16 = 0xFF01
	consumerUsagePage uint16 = 0x000C

	protocolKeyCount                 = 6
	keyLEDConfigSize                 = 4
	macropadReportSize               = 6
	hostMacropadReportSize           = 7
	configKindKeyColors         byte = 0x01
	configReportSize                 = 1 + (protocolKeyCount * keyLEDConfigSize)
	usbVendorInputReportSize         = hostMacropadReportSize
	usbVendorOutputReportSize        = configReportSize
	usbVendorInputTransferSize       = 1 + usbVendorInputReportSize
	usbVendorOutputTransferSize      = 1 + usbVendorOutputReportSize

	desktopBLEProtocolVersion     byte = 0x01
	desktopBLECapabilityLEDConfig byte = 0x01
	desktopBLEProtocolSize             = 2
	desktopBLEInputReportSize          = macropadReportSize
	desktopBLEConfigSize               = configReportSize
	desktopBLEServiceUUID              = "6f7d7a10-5d57-4a0d-9f0d-484150440001"
	desktopBLEProtocolUUID             = "6f7d7a11-5d57-4a0d-9f0d-484150440001"
	desktopBLEInputUUID                = "6f7d7a12-5d57-4a0d-9f0d-484150440001"
	desktopBLEConfigUUID               = "6f7d7a13-5d57-4a0d-9f0d-484150440001"
)

type Source string

const (
	SourceUSB Source = "usb"
	SourceBLE Source = "ble"
)

type KeyLEDSetting struct {
	Color      [3]byte
	Brightness uint8
}

type KeyLEDSettings [protocolKeyCount]KeyLEDSetting

type Info struct {
	Source          Source `json:"source"`
	Path            string `json:"path"`
	VendorID        uint16 `json:"vendorID"`
	ProductID       uint16 `json:"productID"`
	UsagePage       uint16 `json:"usagePage"`
	Usage           uint16 `json:"usage"`
	InterfaceNumber int    `json:"interfaceNumber"`
}

type Report struct {
	Source          Source `json:"source,omitempty"`
	Connected       bool   `json:"connected"`
	Keys            uint8  `json:"keys"`
	EncoderDelta    int8   `json:"encoderDelta"`
	EncoderPressed  bool   `json:"encoderPressed"`
	BatteryMV       uint16 `json:"batteryMV"`
	USBPowerPresent bool   `json:"usbPowerPresent"`
}

type DisconnectInfo struct {
	Source  Source `json:"source,omitempty"`
	Reason  string `json:"reason"`
	IsError bool   `json:"isError"`
}

type Callbacks struct {
	Connected    func(Info)
	Disconnected func(DisconnectInfo)
	Report       func(Report)
	Log          func(string, string)
}
