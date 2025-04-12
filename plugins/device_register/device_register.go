package device_register

// Package device_register provides a plugin for registering devices in the simulator.

import (
	"context"
	"fmt"
	"net/http"

	"github.com/luraproject/lura/v2/config"
	"github.com/luraproject/lura/v2/proxy"
	"github.com/luraproject/lura/v2/vicg"
	"github.com/yuanyuanxiang/fss/pkg/audit"
	cvt "github.com/yuanyuanxiang/fss/pkg/convert"
)

type SessionManager interface {
	VerifyAuthHeader(authHeader string) (string, error)
}

type DeviceManager interface {
	RegisterDevice(serialNumber, publicKey, state string, isVerified bool) error
}

type factory struct {
	sess      SessionManager
	dev       DeviceManager
	publicKey string
}

// Plugin defines
type Plugin struct {
	factory
	name  string
	index int
	log   audit.LogManager
}

func NewFactory(sess SessionManager, dev DeviceManager, publicKey string) vicg.VicgPluginFactory {
	return factory{sess: sess, dev: dev, publicKey: publicKey}
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

/*
Request:

	{
		"serial_number": "1234567890",
		"public_key": "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	}

Response:

	{
		"code" : 0,
		"msg" : "ok"
		"public_key": "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	}
*/
func (p *Plugin) HandleHTTPMessage(ctx context.Context, request *proxy.Request, response *proxy.Response) error {
	// verify auth header
	serialNumber, err := p.sess.VerifyAuthHeader(request.HeaderGet("Authorization"))
	if serialNumber == "" || err != nil {
		response.WriteHeader(http.StatusUnauthorized)
		p.log.AddIncidentLog(request.RemoteAddr, serialNumber, "missing or invalid authorization header", http.StatusUnauthorized, err.Error())
		response.Data = map[string]interface{}{
			"code":          http.StatusUnauthorized,
			"msg":           fmt.Sprintf("missing or invalid authorization header: %v", err),
			"serial_number": serialNumber,
		}
		return p.Error()
	}

	if cvt.ToString(request.Private["serial_number"]) != serialNumber {
		response.WriteHeader(http.StatusBadRequest)
		p.log.AddIncidentLog(request.RemoteAddr, serialNumber, "serial number mismatch", http.StatusBadRequest)
		response.Data = map[string]interface{}{
			"code":          http.StatusBadRequest,
			"msg":           "serial number mismatch",
			"serial_number": cvt.ToString(request.Private["serial_number"]),
		}
		return p.Error()
	}

	// register device: if the allowance is exceeded, it will also return an error
	if err := p.dev.RegisterDevice(serialNumber, cvt.ToString(request.Private["public_key"]),
		cvt.ToString(request.Private["state"]), true); err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		p.log.AddIncidentLog(request.RemoteAddr, serialNumber, "failed to register device", http.StatusInternalServerError, err.Error())
		response.Data = map[string]interface{}{
			"code":          http.StatusInternalServerError,
			"msg":           fmt.Sprintf("failed to register device: %v", err),
			"serial_number": serialNumber,
		}
		return p.Error()
	}

	response.Data = map[string]interface{}{
		"code":          0,
		"msg":           "success",
		"serial_number": serialNumber,
		"public_key":    p.publicKey,
	}
	response.WriteHeader(http.StatusCreated)
	p.log.AddLog(request.RemoteAddr, serialNumber, "success", http.StatusOK)
	return nil
}

func (p *Plugin) Priority() int {
	return p.index
}

func (p *Plugin) Error() error {
	return fmt.Errorf("failed on plugin: '%s'", p.name)
}
