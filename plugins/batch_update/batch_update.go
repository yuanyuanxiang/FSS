package batch_update

// Package batch_update provides a plugin for batch updating devices in a simulator.

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
	BatchUpdate(startSerial, endSerial int) error
}

type factory struct {
	sim DeviceSimulator
}

// Plugin defines
type Plugin struct {
	factory
	name  string
	index int
	infra interface{}
}

func NewFactory(sim DeviceSimulator) vicg.VicgPluginFactory {
	return factory{sim: sim}
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
		"start_serial": 1,
		"end_serial": 100
	}
*/
func (p *Plugin) HandleHTTPMessage(ctx context.Context, request *proxy.Request, response *proxy.Response) error {
	startSerial := cvt.ToInt(request.Private["start_serial"])
	endSerial := cvt.ToInt(request.Private["end_serial"])
	if startSerial <= 0 || endSerial <= 0 {
		response.WriteHeader(http.StatusBadRequest)
		return fmt.Errorf("start_serial and end_serial must be greater than 0")
	}
	err := p.sim.BatchUpdate(startSerial, endSerial)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		return fmt.Errorf("failed to batch devices: %v", err)
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
