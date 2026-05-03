package tray

import (
	"reflect"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
)

var systemTrayCallbackType = reflect.TypeOf((func())(nil))

// Wails does not currently expose SystemTray menu lifecycle hooks, but Linux
// tray menus report "opened" as a click. Wiring these callbacks lets the app
// suppress that synthetic click until the menu closes.
func setSystemTrayMenuHooks(tray *application.SystemTray, onOpen, onClose func()) bool {
	openSet := setSystemTrayCallback(tray, "onMenuOpen", onOpen)
	closeSet := setSystemTrayCallback(tray, "onMenuClose", onClose)
	return openSet && closeSet
}

func setSystemTrayCallback(tray *application.SystemTray, fieldName string, fn func()) bool {
	field, ok := systemTrayCallbackField(tray, fieldName)
	if !ok {
		return false
	}

	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(reflect.ValueOf(fn))
	return true
}

func systemTrayCallbackField(tray *application.SystemTray, fieldName string) (reflect.Value, bool) {
	if tray == nil {
		return reflect.Value{}, false
	}

	value := reflect.ValueOf(tray)
	if value.Kind() != reflect.Pointer || value.IsNil() {
		return reflect.Value{}, false
	}

	elem := value.Elem()
	if elem.Kind() != reflect.Struct {
		return reflect.Value{}, false
	}

	field := elem.FieldByName(fieldName)
	if !field.IsValid() || !field.CanAddr() || field.Type() != systemTrayCallbackType {
		return reflect.Value{}, false
	}

	return field, true
}
