package httpdata_parse

// Pcakage httpdata_parse parse http request body and save to request.Private

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/luraproject/lura/v2/config"
	"github.com/luraproject/lura/v2/proxy"
	"github.com/luraproject/lura/v2/vicg"
)

type factory struct {
}

// Plugin defines
type Plugin struct {
	factory
	name  string
	index int
	infra interface{}
}

func NewFactory() vicg.VicgPluginFactory {
	return factory{}
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
	if request.Body != nil {
		if data, err := io.ReadAll(request.Body); err == nil {
			request.Body = io.NopCloser(bytes.NewBuffer(data))
			return json.Unmarshal(data, &request.Private)
		} else {
			return fmt.Errorf("failed to read request body: %v", err)
		}
	}

	return nil
}

func (p *Plugin) Priority() int {
	return p.index
}
