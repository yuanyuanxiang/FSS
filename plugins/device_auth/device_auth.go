package device_auth

// Package device_auth provides a plugin for device authentication.
// Manually block a specific device or authorize a specific device.
import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/luraproject/lura/v2/config"
	"github.com/luraproject/lura/v2/proxy"
	"github.com/luraproject/lura/v2/vicg"
	"github.com/yuanyuanxiang/fss/pkg/audit"
)

type DeviceAuth interface {
	BlockDevice(serialNumber string) error
	AuthorizeDevice(serialNumber string) error
}

type factory struct {
	auth DeviceAuth
}

// Plugin defines
type Plugin struct {
	factory
	name  string
	index int
	log   audit.LogManager
}

func NewFactory(auth DeviceAuth) vicg.VicgPluginFactory {
	return factory{auth: auth}
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
	arr := strings.Split(request.Path, "/")
	serialNumber := arr[len(arr)-2]
	operation := arr[len(arr)-1]
	var err error
	switch operation {
	case "block":
		err = p.auth.BlockDevice(serialNumber)
	case "authorize":
		err = p.auth.AuthorizeDevice(serialNumber)
	default:
		response.WriteHeader(http.StatusBadRequest)
		response.Data = map[string]interface{}{"code": http.StatusBadRequest, "msg": "invalid operation"}
		return nil
	}
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		p.log.AddIncidentLog(request.RemoteAddr, serialNumber, "failed to "+operation, http.StatusInternalServerError, err)
		response.Data = map[string]interface{}{"code": http.StatusInternalServerError, "msg": err.Error()}
		return nil
	}
	response.Data = map[string]interface{}{
		"code":          0,
		"msg":           "success",
		"serial_number": serialNumber,
		"operation":     operation,
	}
	p.log.AddLog(request.RemoteAddr, serialNumber, "success", http.StatusOK)

	return nil
}

func (p *Plugin) Priority() int {
	return p.index
}
