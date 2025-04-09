package allowance_update

// Package allowance_update provides a plugin for updating device allowances.
import (
	"context"
	"fmt"
	"net/http"

	"github.com/luraproject/lura/v2/config"
	"github.com/luraproject/lura/v2/proxy"
	"github.com/luraproject/lura/v2/vicg"
	cvt "github.com/yuanyuanxiang/fss/pkg/convert"
)

type AllowanceManeger interface {
	IncreaseAllowance(key string, inc int)
}

type factory struct {
	allow AllowanceManeger
}

// Plugin defines
type Plugin struct {
	factory
	name  string
	index int
	infra interface{}
}

func NewFactory(allow AllowanceManeger) vicg.VicgPluginFactory {
	return factory{allow: allow}
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
		"increase_allowance": 10
	}
*/
func (p *Plugin) HandleHTTPMessage(ctx context.Context, request *proxy.Request, response *proxy.Response) error {
	allowanceInc := cvt.ToInt(request.Private["increase_allowance"])
	if allowanceInc <= 0 {
		response.WriteHeader(http.StatusBadRequest)
		return fmt.Errorf("invalid increase_allowance: %d", allowanceInc)
	}
	p.allow.IncreaseAllowance("", allowanceInc)
	response.Data = map[string]interface{}{"code": 0, "msg": "ok"}
	return nil
}

func (p *Plugin) Priority() int {
	return p.index
}
