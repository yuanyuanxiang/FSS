package audit_logs

// Package audit_logs provides a plugin for logging audit information.
// Retrieve logs of successful updates.
// Retrieve logs of security incidents and rejected attempts
import (
	"context"
	"fmt"
	"strings"

	"github.com/luraproject/lura/v2/config"
	"github.com/luraproject/lura/v2/proxy"
	"github.com/luraproject/lura/v2/vicg"
	"github.com/yuanyuanxiang/fss/pkg/audit"
)

type factory struct {
}

// Plugin defines
type Plugin struct {
	factory
	name  string
	index int
	log   audit.LogManager
}

func NewFactory() vicg.VicgPluginFactory {
	return factory{}
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
	typ := request.Path[strings.LastIndex(request.Path, "/")+1:]
	arr, err := p.log.GetAuditLogs(typ)
	response.Data["msg"] = "success"
	response.Data["code"] = 0
	if err != nil {
		response.Data["code"] = -1
		response.Data["msg"] = fmt.Sprintf("failed to get audit logs: %v", err)
	}
	response.Data["audit_logs"] = arr
	response.Data["type"] = typ
	response.Data["count"] = len(arr)

	return nil
}

func (p *Plugin) Priority() int {
	return p.index
}
