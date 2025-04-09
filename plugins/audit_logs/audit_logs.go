package audit_logs

// Package audit_logs provides a plugin for logging audit information.
// Retrieve logs of successful updates.
// Retrieve logs of security incidents and rejected attempts
import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/luraproject/lura/v2/config"
	"github.com/luraproject/lura/v2/proxy"
	"github.com/luraproject/lura/v2/vicg"
)

type AuditLog interface {
	GetAuditLogs(typ string) ([]map[string]interface{}, error)
}

type factory struct {
	audit AuditLog
}

// Plugin defines
type Plugin struct {
	factory
	name  string
	index int
	infra interface{}
}

func NewFactory(audit AuditLog) vicg.VicgPluginFactory {
	return factory{audit: audit}
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
	typ := request.Path[strings.LastIndex(request.Path, "/")+1:]
	arr, err := p.audit.GetAuditLogs(typ)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		return fmt.Errorf("failed to get audit logs: %v", err)
	}
	response.Data["audit_logs"] = arr
	response.Data["type"] = typ
	response.Data["count"] = len(arr)
	response.Data["msg"] = "success"
	response.Data["code"] = 0

	return nil
}

func (p *Plugin) Priority() int {
	return p.index
}
