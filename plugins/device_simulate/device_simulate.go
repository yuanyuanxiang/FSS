package device_simulate

// Package device_simulate provides a plugin for generating devices in the simulator.

import (
	"context"
	"fmt"
	"net/http"

	"github.com/luraproject/lura/v2/config"
	"github.com/luraproject/lura/v2/proxy"
	"github.com/luraproject/lura/v2/vicg"
	cvt "github.com/yuanyuanxiang/fss/pkg/convert"
)

type DeviceSimulator interface {
	GenerateDevices(master string, count int, startSerial int) error
}

type factory struct {
	gen DeviceSimulator
}

// Plugin defines
type Plugin struct {
	factory
	name  string
	index int
	infra interface{}
}

func NewFactory(gen DeviceSimulator) vicg.VicgPluginFactory {
	return factory{gen: gen}
}

func (f factory) New(cfg *config.PluginConfig, infra interface{}) (vicg.VicgPlugin, error) {
	return &Plugin{
		factory: f,
		index:   cfg.Index,
		name:    cfg.Name,
		infra:   infra,
	}, nil
}

/*
Request:

	{
		"master_address": "127.0.0.1:9000",
		"generate": "100",
		"start-serial":1
	}
*/
func (p *Plugin) HandleHTTPMessage(ctx context.Context, request *proxy.Request, response *proxy.Response) error {
	masterAddress := cvt.ToString(request.Private["master_address"])
	generate := cvt.ToInt(request.Private["generate"])
	startSerial := cvt.ToInt(request.Private["start-serial"])
	if generate <= 0 || startSerial <= 0 {
		response.WriteHeader(http.StatusBadRequest)
		return fmt.Errorf("generate and start-serial must be greater than 0")
	}
	err := p.gen.GenerateDevices(masterAddress, generate, startSerial)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		return fmt.Errorf("failed to generate devices: %v", err)
	}
	response.Data = map[string]interface{}{
		"code": 0,
		"msg":  "success",
	}

	return nil
}

func (p *Plugin) Priority() int {
	return p.index
}
