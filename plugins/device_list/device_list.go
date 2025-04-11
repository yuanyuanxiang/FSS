package device_list

// Package device_list provides a plugin for listing devices in the simulator.

import (
	"context"
	"fmt"
	"net/http"

	"github.com/luraproject/lura/v2/config"
	"github.com/luraproject/lura/v2/proxy"
	"github.com/luraproject/lura/v2/vicg"
	"github.com/yuanyuanxiang/fss/pkg/audit"
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
	log   audit.LogManager
}

func NewFactory(dev DeviceQuery) vicg.VicgPluginFactory {
	return factory{dev: dev}
}

func (f factory) New(cfg *config.PluginConfig, infra interface{}) (vicg.VicgPlugin, error) {
	p := &Plugin{
		factory: f,
		index:   cfg.Index,
		name:    cfg.Name,
		log:     nil,
	}
	var m map[string]interface{}
	if v, ok := infra.(*vicg.Infra); ok && v != nil {
		m = v.ExtraConfig
	}
	p.log, _ = m[audit.LOG_MANAGER].(audit.LogManager)
	if p.log == nil {
		return nil, fmt.Errorf("audit log manager is not set")
	}
	return p, nil
}

func (p *Plugin) HandleHTTPMessage(ctx context.Context, request *proxy.Request, response *proxy.Response) error {
	arr, err := p.dev.GetDeviceList()
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Data = map[string]interface{}{
			"code": http.StatusInternalServerError,
			"msg":  fmt.Sprintf("failed to list devices: %v", err),
		}
		return nil
	}
	response.Data["devices"] = arr
	response.Data["code"] = 0
	response.Data["msg"] = "success"
	response.Data["total"] = len(arr)

	return nil
}

func (p *Plugin) Priority() int {
	return p.index
}
