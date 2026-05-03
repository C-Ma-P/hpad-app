package device

func encodeKeyLEDConfigReport(settings KeyLEDSettings) [usbVendorOutputTransferSize]byte {
	var report [usbVendorOutputTransferSize]byte
	offset := 2

	report[0] = 0
	report[1] = configKindKeyColors
	for _, setting := range settings {
		copy(report[offset:offset+len(setting.Color)], setting.Color[:])
		offset += len(setting.Color)
		report[offset] = setting.Brightness
		offset++
	}

	return report
}

func decodeVendorInputReport(raw []byte) (Report, bool) {
	payload := raw
	switch {
	case len(raw) >= usbVendorInputTransferSize && raw[0] == 0:
		payload = raw[1:usbVendorInputTransferSize]
	case len(raw) == 5 && raw[0] == 0:
		payload = raw[1:5]
	case len(raw) == 4 && raw[0] == 0 && !(raw[1] == 0 && raw[2] == 0 && raw[3] == 0):
		payload = raw[1:]
	}

	switch {
	case len(payload) >= hostMacropadReportSize:
		return Report{
			Connected:       payload[0] != 0,
			Keys:            payload[1],
			EncoderDelta:    int8(payload[2]),
			EncoderPressed:  payload[3] != 0,
			BatteryMV:       uint16(payload[4]) | (uint16(payload[5]) << 8),
			USBPowerPresent: payload[6] != 0,
		}, true
	case len(payload) >= 4:
		return Report{
			Connected:      payload[0] != 0,
			Keys:           payload[1],
			EncoderDelta:   int8(payload[2]),
			EncoderPressed: payload[3] != 0,
		}, true
	case len(payload) < 3:
		return Report{}, false
	default:
		return Report{
			Connected:      true,
			Keys:           payload[0],
			EncoderDelta:   int8(payload[1]),
			EncoderPressed: payload[2] != 0,
		}, true
	}
}
