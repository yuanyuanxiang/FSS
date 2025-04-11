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
	BatchUpdate(startSerial, endSerial int, version string) error
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
		"end_serial": 100,
		"version": "1.0.1"
	}
*/
func (p *Plugin) HandleHTTPMessage(ctx context.Context, request *proxy.Request, response *proxy.Response) error {
	startSerial := cvt.ToInt(request.Private["start_serial"])
	endSerial := cvt.ToInt(request.Private["end_serial"])
	if startSerial < 0 || endSerial < 0 {
		response.WriteHeader(http.StatusBadRequest)
		response.Data = map[string]interface{}{
			"code": http.StatusBadRequest,
			"msg":  "start_serial and end_serial must be greater than 0",
		}
		return nil
	}
	if endSerial < startSerial {
		response.WriteHeader(http.StatusBadRequest)
		response.Data = map[string]interface{}{
			"code": http.StatusBadRequest,
			"msg":  "end_serial must be greater than start_serial",
		}
		return nil
	}
	version := cvt.ToString(request.Private["version"])
	if version == "" {
		response.WriteHeader(http.StatusBadRequest)
		response.Data = map[string]interface{}{
			"code": http.StatusBadRequest,
			"msg":  "version is required",
		}
		return nil
	}
	err := p.sim.BatchUpdate(startSerial, endSerial, version)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Data = map[string]interface{}{
			"code": http.StatusInternalServerError,
			"msg":  fmt.Sprintf("failed to batch update: %v", err),
		}
		return nil
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
