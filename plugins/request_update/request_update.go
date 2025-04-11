package request_update

// Package request_update provides a plugin for updating devices in a simulator.

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/luraproject/lura/v2/config"
	"github.com/luraproject/lura/v2/proxy"
	"github.com/luraproject/lura/v2/vicg"
	cvt "github.com/yuanyuanxiang/fss/pkg/convert"
)

type DeviceSimulator interface {
	UpdateDevice(startSerial int, version string) error
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

func (p *Plugin) HandleHTTPMessage(ctx context.Context, request *proxy.Request, response *proxy.Response) error {
	arr := strings.Split(request.URL.Path, "/")
	serialNumber := arr[len(arr)-2]
	if serialNumber == "" {
		response.WriteHeader(http.StatusBadRequest)
		response.Data = map[string]interface{}{
			"code": http.StatusBadRequest,
			"msg":  "serial number is required",
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
	err := p.sim.UpdateDevice(cvt.ToInt(serialNumber), version)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Data = map[string]interface{}{
			"code": http.StatusInternalServerError,
			"msg":  fmt.Sprintf("failed to update devices: %v", err),
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
