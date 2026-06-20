package xray

import (
	"bytes"
	"fmt"

	xcore "github.com/xtls/xray-core/core"
	"github.com/xtls/xray-core/infra/conf/serial"

	// Register all inbound/outbound/transport/app implementations.
	_ "github.com/xtls/xray-core/main/distro/all"
)

// Instance is a running embedded xray-core instance.
type Instance struct {
	inst *xcore.Instance
}

// Start loads a JSON config and starts an embedded xray-core instance. The
// instance keeps running until Close is called.
func Start(configJSON []byte) (*Instance, error) {
	cfg, err := serial.LoadJSONConfig(bytes.NewReader(configJSON))
	if err != nil {
		return nil, fmt.Errorf("xray: load config: %w", err)
	}
	inst, err := xcore.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("xray: create instance: %w", err)
	}
	if err := inst.Start(); err != nil {
		return nil, fmt.Errorf("xray: start instance: %w", err)
	}
	return &Instance{inst: inst}, nil
}

// Close stops the instance and releases its resources.
func (i *Instance) Close() error {
	if i == nil || i.inst == nil {
		return nil
	}
	return i.inst.Close()
}
