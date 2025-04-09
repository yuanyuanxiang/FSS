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
	infra interface{}
}

func NewFactory(auth DeviceAuth) vicg.VicgPluginFactory {
	return factory{auth: auth}
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
		response.Data = map[string]interface{}{"code": 400, "msg": "invalid operation"}
		return fmt.Errorf("invalid operation: %s", operation)
	}
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Data = map[string]interface{}{"code": 500, "msg": err.Error()}
		return fmt.Errorf("failed to %s device: %v", operation, err)
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
