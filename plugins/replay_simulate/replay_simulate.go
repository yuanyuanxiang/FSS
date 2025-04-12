package replay_simulate

// Package replay_simulate provides a plugin for simulating device replay in the simulator.

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
	Replay(startSerial int) error
	BatchReplay(startSerial, endSerial int) error
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
		"end_serial": ""
	}
*/
func (p *Plugin) HandleHTTPMessage(ctx context.Context, request *proxy.Request, response *proxy.Response) error {
	serialNumber := request.URL.Path[strings.LastIndex(request.URL.Path, "/")+1:]
	if serialNumber == "" {
		response.WriteHeader(http.StatusBadRequest)
		response.Data = map[string]interface{}{
			"code": http.StatusBadRequest,
			"msg":  "serial number is required",
		}
		return p.Error()
	}
	var err error
	startSerial := cvt.ToInt(serialNumber)
	endSerial := cvt.ToInt(request.Private["end_serial"])
	if endSerial > startSerial { // batch replay
		err = p.sim.BatchReplay(startSerial, endSerial)
	} else { // single replay
		err = p.sim.Replay(cvt.ToInt(serialNumber))
	}
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Data = map[string]interface{}{
			"code":          http.StatusInternalServerError,
			"msg":           fmt.Sprintf("failed to replay devices: %v", err),
			"serial_number": serialNumber,
		}
		return p.Error()
	}
	response.Data = map[string]interface{}{
		"code":          0,
		"msg":           "success",
		"serial_number": serialNumber,
	}
	return nil
}

func (p *Plugin) Priority() int {
	return p.index
}

func (p *Plugin) Error() error {
	return fmt.Errorf("failed on plugin: '%s'", p.name)
}
