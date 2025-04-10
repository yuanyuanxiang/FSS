package device_register

// Package device_register provides a plugin for registering devices in the simulator.

import (
	"context"
	"fmt"
	"net/http"

	"github.com/luraproject/lura/v2/config"
	"github.com/luraproject/lura/v2/proxy"
	"github.com/luraproject/lura/v2/vicg"
	cvt "github.com/yuanyuanxiang/fss/pkg/convert"
)

type SessionManager interface {
	VerifyAuthHeader(authHeader string) (string, error)
}

type DeviceManager interface {
	IsDeviceRegistered(serialNumber string) bool
	RegisterDevice(serialNumber, publicKey string, isVerified bool)
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
	infra interface{}
}

func NewFactory(sess SessionManager, dev DeviceManager, publicKey string) vicg.VicgPluginFactory {
	return factory{sess: sess, dev: dev, publicKey: publicKey}
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
		return fmt.Errorf("missing or invalid authorization header: %v", err)
	}

	if cvt.ToString(request.Private["serial_number"]) != serialNumber {
		response.WriteHeader(http.StatusBadRequest)
		return fmt.Errorf("serial number mismatch")
	}

	// check if device is already registered
	if p.dev.IsDeviceRegistered(serialNumber) {
		response.WriteHeader(http.StatusConflict)
		return fmt.Errorf("device already registered")
	}

	// register device
	p.dev.RegisterDevice(serialNumber, cvt.ToString(request.Private["public_key"]), true)

	response.Data = map[string]interface{}{
		"code":          0,
		"msg":           "ok",
		"serial_number": serialNumber,
		"public_key":    p.publicKey,
	}
	response.WriteHeader(http.StatusCreated)

	return nil
}

func (p *Plugin) Priority() int {
	return p.index
}
