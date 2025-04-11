package challenge_gen

// Package challenge_gen provides a plugin for generating challenges for device authentication.

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/luraproject/lura/v2/config"
	"github.com/luraproject/lura/v2/proxy"
	"github.com/luraproject/lura/v2/vicg"
	"github.com/yuanyuanxiang/fss/internal/pkg/common"
)

type SessionManager interface {
	AddSess(serialNumber, challenge string, expiresAt time.Time, isVerified bool)
}

type factory struct {
	sess SessionManager
}

// Plugin defines
type Plugin struct {
	factory
	name  string
	index int
	infra interface{}
}

func NewFactory(sess SessionManager) vicg.VicgPluginFactory {
	return factory{sess: sess}
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
Response:

	{
		"serial_number": "1234567890",
		"challenge": "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		"expiresIn": "5m"
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
		return nil
	}

	challenge := common.GenerateChallenge()
	expiresAt := time.Now().Add(5 * time.Minute)
	// prepare a session alive for 5 minutes
	// and set it to not verified
	// the sess id is the serial number + challenge
	p.sess.AddSess(serialNumber, challenge, expiresAt, false)

	response.Data = map[string]interface{}{
		"serial_number": serialNumber,
		"challenge":     challenge,
		"expiresIn":     "5m",
		"code":          0,
		"msg":           "ok",
	}

	return nil
}

func (p *Plugin) Priority() int {
	return p.index
}
