package device_list

// Package device_list provides a plugin for listing devices in the simulator.

import (
	"context"
	"fmt"
	"net/http"

	"github.com/luraproject/lura/v2/config"
	"github.com/luraproject/lura/v2/proxy"
	"github.com/luraproject/lura/v2/vicg"
)

type DeviceQuery interface {
	GetDeviceList() ([]map[string]interface{}, error)
}

type factory struct {
	dev DeviceQuery
}

// Plugin defines
type Plugin struct {
	factory
	name  string
	index int
	infra interface{}
}

func NewFactory(dev DeviceQuery) vicg.VicgPluginFactory {
	return factory{dev: dev}
}

func (f factory) New(cfg *config.PluginConfig, infra interface{}) (vicg.VicgPlugin, error) {
	return &Plugin{
		factory: f,
		index:   cfg.Index,
		name:    cfg.Name,
		infra:   infra,
	}, nil
}

func (p *Plugin) HandleHTTPMessage(ctx context.Context, request *proxy.Request, response *proxy.Response) error {
	arr, err := p.dev.GetDeviceList()
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		return fmt.Errorf("failed to list devices: %v", err)
	}
	response.Data["devices"] = arr
	response.Data["code"] = 0
	response.Data["msg"] = "success"

	return nil
}

func (p *Plugin) Priority() int {
	return p.index
}
