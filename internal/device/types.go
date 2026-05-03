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
)

type KeyLEDSetting struct {
	Color      [3]byte
	Brightness uint8
}

type KeyLEDSettings [protocolKeyCount]KeyLEDSetting

type Info struct {
	Path            string `json:"path"`
	VendorID        uint16 `json:"vendorID"`
	ProductID       uint16 `json:"productID"`
	UsagePage       uint16 `json:"usagePage"`
	Usage           uint16 `json:"usage"`
	InterfaceNumber int    `json:"interfaceNumber"`
}

type Report struct {
	Connected       bool   `json:"connected"`
	Keys            uint8  `json:"keys"`
	EncoderDelta    int8   `json:"encoderDelta"`
	EncoderPressed  bool   `json:"encoderPressed"`
	BatteryMV       uint16 `json:"batteryMV"`
	USBPowerPresent bool   `json:"usbPowerPresent"`
}

type DisconnectInfo struct {
	Reason  string `json:"reason"`
	IsError bool   `json:"isError"`
}

type Callbacks struct {
	Connected    func(Info)
	Disconnected func(DisconnectInfo)
	Report       func(Report)
	Log          func(string, string)
}
