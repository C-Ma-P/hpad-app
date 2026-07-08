//go:build linux

package device

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
)

const bluezService = "org.bluez"

type desktopBLEClient struct {
	callbacks  Callbacks
	emitReport func(Source, Report)

	mu         sync.Mutex
	conn       *dbus.Conn
	devicePath dbus.ObjectPath
	configPath dbus.ObjectPath
	connected  bool
}

func newDesktopBLEClient(callbacks Callbacks, emitReport func(Source, Report)) *desktopBLEClient {
	return &desktopBLEClient{callbacks: callbacks, emitReport: emitReport}
}

func (c *desktopBLEClient) Start(ctx context.Context) error {
	for {
		if ctx.Err() != nil {
			return nil
		}
		if err := c.runOnce(ctx); err != nil {
			c.callbacks.Log("warn", fmt.Sprintf("Desktop BLE disconnected: %v", err))
			c.markDisconnected(err)
		}
		if !waitRetry(ctx) {
			return nil
		}
	}
}

func (c *desktopBLEClient) Stop() {
	c.mu.Lock()
	conn := c.conn
	devicePath := c.devicePath
	c.mu.Unlock()

	if conn != nil && devicePath != "" {
		_ = conn.Object(bluezService, devicePath).Call("org.bluez.Device1.Disconnect", 0).Err
	}
	c.markDisconnected(nil)
}

func (c *desktopBLEClient) SyncKeyLEDSettings(settings KeyLEDSettings) error {
	payload := encodeKeyLEDConfigPayload(settings)

	c.mu.Lock()
	conn := c.conn
	configPath := c.configPath
	connected := c.connected
	c.mu.Unlock()

	if !connected || conn == nil || configPath == "" {
		return errors.New("Desktop BLE macropad is not connected")
	}

	return conn.Object(bluezService, configPath).
		Call("org.bluez.GattCharacteristic1.WriteValue", 0, payload[:], map[string]dbus.Variant{}).Err
}

func (c *desktopBLEClient) runOnce(ctx context.Context) error {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return fmt.Errorf("connect system bus: %w", err)
	}
	defer conn.Close()

	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		if c.conn == conn {
			c.conn = nil
			c.configPath = ""
			c.devicePath = ""
		}
		c.mu.Unlock()
	}()

	adapterPath, err := findAdapter(conn)
	if err != nil {
		return err
	}
	adapter := conn.Object(bluezService, adapterPath)
	filter := map[string]dbus.Variant{
		"UUIDs":     dbus.MakeVariant([]string{desktopBLEServiceUUID}),
		"Transport": dbus.MakeVariant("le"),
	}
	if err := adapter.Call("org.bluez.Adapter1.SetDiscoveryFilter", 0, filter).Err; err != nil {
		return fmt.Errorf("set BLE discovery filter: %w", err)
	}
	if err := adapter.Call("org.bluez.Adapter1.StartDiscovery", 0).Err; err != nil {
		return fmt.Errorf("start BLE discovery: %w", err)
	}
	defer adapter.Call("org.bluez.Adapter1.StopDiscovery", 0)

	devicePath, err := waitForDesktopBLEDevice(ctx, conn)
	if err != nil {
		return err
	}

	device := conn.Object(bluezService, devicePath)
	if err := device.Call("org.bluez.Device1.Connect", 0).Err; err != nil {
		return fmt.Errorf("connect Desktop BLE macropad: %w", err)
	}
	defer device.Call("org.bluez.Device1.Disconnect", 0)

	if err := waitForServicesResolved(ctx, conn, devicePath); err != nil {
		return err
	}

	protocolPath, inputPath, configPath, err := findDesktopBLECharacteristics(conn, devicePath)
	if err != nil {
		return err
	}
	protocol, err := readCharacteristic(conn, protocolPath)
	if err != nil {
		return fmt.Errorf("read Desktop BLE protocol: %w", err)
	}
	if !decodeDesktopBLEProtocol(protocol) {
		return fmt.Errorf("unsupported Desktop BLE protocol payload % X", protocol)
	}

	signalCh := make(chan *dbus.Signal, 16)
	conn.Signal(signalCh)
	defer conn.RemoveSignal(signalCh)
	matchOptions := []dbus.MatchOption{
		dbus.WithMatchInterface("org.freedesktop.DBus.Properties"),
		dbus.WithMatchMember("PropertiesChanged"),
	}
	if err := conn.AddMatchSignal(matchOptions...); err != nil {
		return fmt.Errorf("subscribe BlueZ property changes: %w", err)
	}
	defer conn.RemoveMatchSignal(matchOptions...)

	if err := conn.Object(bluezService, inputPath).Call("org.bluez.GattCharacteristic1.StartNotify", 0).Err; err != nil {
		return fmt.Errorf("start Desktop BLE notifications: %w", err)
	}
	defer conn.Object(bluezService, inputPath).Call("org.bluez.GattCharacteristic1.StopNotify", 0)

	c.mu.Lock()
	c.devicePath = devicePath
	c.configPath = configPath
	c.connected = true
	c.mu.Unlock()
	c.callbacks.Connected(Info{Source: SourceBLE, Path: string(devicePath)})
	c.callbacks.Log("info", fmt.Sprintf("Connected Desktop BLE macropad at %s", devicePath))
	defer c.markDisconnected(errors.New("Desktop BLE link closed"))

	for {
		select {
		case <-ctx.Done():
			return nil
		case sig := <-signalCh:
			if sig == nil {
				continue
			}
			if sig.Path == inputPath {
				c.handleInputSignal(sig)
			}
			if sig.Path == devicePath && deviceDisconnectedSignal(sig) {
				return errors.New("Desktop BLE device disconnected")
			}
		case <-time.After(2 * time.Second):
			connected, err := getBoolProperty(conn, devicePath, "org.bluez.Device1", "Connected")
			if err != nil || !connected {
				return errors.New("Desktop BLE device disconnected")
			}
		}
	}
}

func (c *desktopBLEClient) handleInputSignal(sig *dbus.Signal) {
	if len(sig.Body) < 2 {
		return
	}
	iface, ok := sig.Body[0].(string)
	if !ok || iface != "org.bluez.GattCharacteristic1" {
		return
	}
	changed, ok := sig.Body[1].(map[string]dbus.Variant)
	if !ok {
		return
	}
	value, ok := variantBytes(changed["Value"])
	if !ok {
		return
	}
	report, ok := decodeDesktopBLEInputReport(value)
	if !ok {
		c.callbacks.Log("warn", fmt.Sprintf("Rejected malformed Desktop BLE input payload % X", value))
		return
	}
	c.emitReport(SourceBLE, report)
}

func (c *desktopBLEClient) markDisconnected(err error) {
	c.mu.Lock()
	wasConnected := c.connected
	c.connected = false
	c.configPath = ""
	c.devicePath = ""
	c.mu.Unlock()

	if wasConnected {
		reason := "Desktop BLE disconnected"
		isError := false
		if err != nil {
			reason = err.Error()
			isError = true
		}
		c.emitReport(SourceBLE, Report{Connected: false})
		c.callbacks.Disconnected(DisconnectInfo{Source: SourceBLE, Reason: reason, IsError: isError})
	}
}

func findAdapter(conn *dbus.Conn) (dbus.ObjectPath, error) {
	objects, err := managedObjects(conn)
	if err != nil {
		return "", err
	}
	for path, ifaces := range objects {
		if _, ok := ifaces["org.bluez.Adapter1"]; ok {
			return path, nil
		}
	}
	return "", errors.New("BlueZ adapter not found")
}

func waitForDesktopBLEDevice(ctx context.Context, conn *dbus.Conn) (dbus.ObjectPath, error) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		path, ok, err := findDesktopBLEDevice(conn)
		if err != nil {
			return "", err
		}
		if ok {
			return path, nil
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
		}
	}
}

func findDesktopBLEDevice(conn *dbus.Conn) (dbus.ObjectPath, bool, error) {
	objects, err := managedObjects(conn)
	if err != nil {
		return "", false, err
	}
	for path, ifaces := range objects {
		props, ok := ifaces["org.bluez.Device1"]
		if !ok {
			continue
		}
		if uuidListContains(props["UUIDs"], desktopBLEServiceUUID) {
			return path, true, nil
		}
	}
	return "", false, nil
}

func waitForServicesResolved(ctx context.Context, conn *dbus.Conn, devicePath dbus.ObjectPath) error {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		resolved, err := getBoolProperty(conn, devicePath, "org.bluez.Device1", "ServicesResolved")
		if err == nil && resolved {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func findDesktopBLECharacteristics(conn *dbus.Conn, devicePath dbus.ObjectPath) (dbus.ObjectPath, dbus.ObjectPath, dbus.ObjectPath, error) {
	objects, err := managedObjects(conn)
	if err != nil {
		return "", "", "", err
	}

	var protocolPath, inputPath, configPath dbus.ObjectPath
	prefix := string(devicePath) + "/"
	for path, ifaces := range objects {
		if !strings.HasPrefix(string(path), prefix) {
			continue
		}
		props, ok := ifaces["org.bluez.GattCharacteristic1"]
		if !ok {
			continue
		}
		uuid, _ := props["UUID"].Value().(string)
		switch strings.ToLower(uuid) {
		case desktopBLEProtocolUUID:
			protocolPath = path
		case desktopBLEInputUUID:
			inputPath = path
		case desktopBLEConfigUUID:
			configPath = path
		}
	}

	if protocolPath == "" || inputPath == "" || configPath == "" {
		return "", "", "", errors.New("Desktop BLE service is missing required characteristics")
	}
	return protocolPath, inputPath, configPath, nil
}

func managedObjects(conn *dbus.Conn) (map[dbus.ObjectPath]map[string]map[string]dbus.Variant, error) {
	var objects map[dbus.ObjectPath]map[string]map[string]dbus.Variant
	err := conn.Object(bluezService, "/").Call("org.freedesktop.DBus.ObjectManager.GetManagedObjects", 0).Store(&objects)
	return objects, err
}

func readCharacteristic(conn *dbus.Conn, path dbus.ObjectPath) ([]byte, error) {
	var value []byte
	err := conn.Object(bluezService, path).
		Call("org.bluez.GattCharacteristic1.ReadValue", 0, map[string]dbus.Variant{}).Store(&value)
	return value, err
}

func getBoolProperty(conn *dbus.Conn, path dbus.ObjectPath, iface string, prop string) (bool, error) {
	value, err := conn.Object(bluezService, path).GetProperty(iface + "." + prop)
	if err != nil {
		return false, err
	}
	boolValue, ok := value.Value().(bool)
	if !ok {
		return false, fmt.Errorf("%s.%s is not bool", iface, prop)
	}
	return boolValue, nil
}

func deviceDisconnectedSignal(sig *dbus.Signal) bool {
	if len(sig.Body) < 2 {
		return false
	}
	iface, ok := sig.Body[0].(string)
	if !ok || iface != "org.bluez.Device1" {
		return false
	}
	changed, ok := sig.Body[1].(map[string]dbus.Variant)
	if !ok {
		return false
	}
	connected, ok := changed["Connected"].Value().(bool)
	return ok && !connected
}

func uuidListContains(value dbus.Variant, want string) bool {
	uuids, ok := value.Value().([]string)
	if !ok {
		return false
	}
	for _, uuid := range uuids {
		if strings.EqualFold(uuid, want) {
			return true
		}
	}
	return false
}

func variantBytes(value dbus.Variant) ([]byte, bool) {
	switch typed := value.Value().(type) {
	case []byte:
		return typed, true
	default:
		return nil, false
	}
}
