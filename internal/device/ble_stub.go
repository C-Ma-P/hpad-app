//go:build !linux

package device

import (
	"context"
	"errors"
)

type desktopBLEClient struct{}

func newDesktopBLEClient(Callbacks, func(Source, Report)) *desktopBLEClient {
	return &desktopBLEClient{}
}

func (c *desktopBLEClient) Start(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func (c *desktopBLEClient) Stop() {}

func (c *desktopBLEClient) SyncKeyLEDSettings(KeyLEDSettings) error {
	return errors.New("Desktop BLE is only implemented on Linux")
}
