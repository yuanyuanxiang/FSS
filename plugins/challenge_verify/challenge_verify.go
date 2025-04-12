package challenge_verify

// Package challenge_verify provides a plugin for verifying HMAC signatures for device authentication.

import (
	"context"
	"fmt"
	"net/http"

	"github.com/luraproject/lura/v2/config"
	"github.com/luraproject/lura/v2/proxy"
	"github.com/luraproject/lura/v2/vicg"
	"github.com/yuanyuanxiang/fss/internal/pkg/common"
	"github.com/yuanyuanxiang/fss/pkg/audit"
	cvt "github.com/yuanyuanxiang/fss/pkg/convert"
)

type SessionManager interface {
	IsValidSess(serialNumber, challenge string) bool
	MarkSessVerified(serialNumber, challenge string) bool
	GenerateAuthHeader(serialNumber string) string
}

type DeviceManager interface {
	GetAllowance(key string) int
}

type factory struct {
	sess   SessionManager
	allow  DeviceManager
	secret string
}

// Plugin defines
type Plugin struct {
	factory
	name  string
	index int
	log   audit.LogManager
}

func NewFactory(sess SessionManager, allow DeviceManager, secret string) vicg.VicgPluginFactory {
	return factory{
		sess:   sess,
		allow:  allow,
		secret: secret,
	}
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
		"signature": "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		"challenge": "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	}

Response:

	{
		"serial_number": "1234567890",
		"status": "verified",
		"token": "xxxxxx"
	}
*/
func (p *Plugin) HandleHTTPMessage(ctx context.Context, request *proxy.Request, response *proxy.Response) error {
	allowance := p.allow.GetAllowance(p.secret)
	serialNumber := cvt.ToString(request.Private["serial_number"])
	if allowance <= 0 {
		response.WriteHeader(http.StatusForbidden)
		p.log.AddIncidentLog(request.RemoteAddr, serialNumber, "allowance exceeded", http.StatusForbidden)
		response.Data = map[string]interface{}{
			"code":          http.StatusForbidden,
			"msg":           "allowance exceeded",
			"serial_number": serialNumber,
		}
		return p.Error()
	}
	signature := cvt.ToString(request.Private["signature"])
	challenge := cvt.ToString(request.Private["challenge"])

	// This is a simple example of signature verification.
	// In a real-world scenario, you would use a more secure method to verify the signature.
	valid := common.VerifySignature(challenge, p.secret, signature)
	if !valid {
		response.WriteHeader(http.StatusUnauthorized)
		p.log.AddIncidentLog(request.RemoteAddr, serialNumber, "invalid signature", http.StatusUnauthorized)
		response.Data = map[string]interface{}{
			"code":          http.StatusUnauthorized,
			"msg":           "invalid signature",
			"serial_number": serialNumber,
		}
		return p.Error()
	}

	if !p.sess.IsValidSess(serialNumber, challenge) {
		response.WriteHeader(http.StatusBadRequest)
		p.log.AddIncidentLog(request.RemoteAddr, serialNumber, "invalid or expired session", http.StatusBadRequest)
		response.Data = map[string]interface{}{
			"code":          http.StatusBadRequest,
			"msg":           "invalid or expired session",
			"serial_number": serialNumber,
		}
		return p.Error()
	}

	// mark the session as verified. only can verify once.
	if !p.sess.MarkSessVerified(serialNumber, challenge) {
		response.WriteHeader(http.StatusBadRequest)
		p.log.AddIncidentLog(request.RemoteAddr, serialNumber, "invalid or verified session", http.StatusBadRequest)
		response.Data = map[string]interface{}{
			"code":          http.StatusBadRequest,
			"msg":           "invalid or verified session",
			"serial_number": serialNumber,
		}
		return p.Error()
	}

	response.Data = map[string]interface{}{
		"code":          0,
		"msg":           "success",
		"serial_number": serialNumber,
		"status":        "verified",
		"token":         p.sess.GenerateAuthHeader(serialNumber),
	}
	p.log.AddLog(request.RemoteAddr, serialNumber, "success", http.StatusOK)
	return nil
}

func (p *Plugin) Priority() int {
	return p.index
}

func (p *Plugin) Error() error {
	return fmt.Errorf("failed on plugin: '%s'", p.name)
}
