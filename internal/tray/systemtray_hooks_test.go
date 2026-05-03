package tray

import (
	"reflect"
	"testing"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func TestSetSystemTrayMenuHooks(t *testing.T) {
	tray := &application.SystemTray{}
	opened := 0
	closed := 0

	if !setSystemTrayMenuHooks(tray, func() { opened++ }, func() { closed++ }) {
		t.Fatal("expected system tray menu hooks to be set")
	}

	openHook, ok := testSystemTrayCallback(tray, "onMenuOpen")
	if !ok {
		t.Fatal("expected onMenuOpen hook to be readable")
	}
	closeHook, ok := testSystemTrayCallback(tray, "onMenuClose")
	if !ok {
		t.Fatal("expected onMenuClose hook to be readable")
	}

	openHook()
	closeHook()

	if opened != 1 {
		t.Fatalf("opened count = %d, want 1", opened)
	}
	if closed != 1 {
		t.Fatalf("closed count = %d, want 1", closed)
	}
}

func testSystemTrayCallback(tray *application.SystemTray, fieldName string) (func(), bool) {
	field, ok := systemTrayCallbackField(tray, fieldName)
	if !ok {
		return nil, false
	}

	value := reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem()
	callback, ok := value.Interface().(func())
	return callback, ok && callback != nil
}
