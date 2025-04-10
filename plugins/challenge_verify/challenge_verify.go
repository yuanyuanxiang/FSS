package challenge_verify

// Package challenge_verify provides a plugin for verifying HMAC signatures for device authentication.

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"

	"github.com/luraproject/lura/v2/config"
	"github.com/luraproject/lura/v2/proxy"
	"github.com/luraproject/lura/v2/vicg"
	"github.com/yuanyuanxiang/fss/internal/pkg/common"
	cvt "github.com/yuanyuanxiang/fss/pkg/convert"
)

type SessionManager interface {
	IsValidSess(serialNumber, challenge string) bool
	MarkSessVerified(serialNumber, challenge string) bool
	GenerateAuthHeader(serialNumber string) string
}

type AllowanceManeger interface {
	GetAllowance(key string) int
}

type factory struct {
	sess   SessionManager
	allow  AllowanceManeger
	secret string
	count  int32
}

// Plugin defines
type Plugin struct {
	factory
	name  string
	index int
	infra interface{}
}

func NewFactory(sess SessionManager, allow AllowanceManeger, secret string) vicg.VicgPluginFactory {
	return factory{
		sess:   sess,
		allow:  allow,
		secret: secret,
	}
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
	num := atomic.LoadInt32(&p.count)
	if num >= int32(allowance) {
		response.WriteHeader(http.StatusTooManyRequests)
		return fmt.Errorf("too many requests")
	}
	serialNumber := cvt.ToString(request.Private["serial_number"])
	signature := cvt.ToString(request.Private["signature"])
	challenge := cvt.ToString(request.Private["challenge"])

	// This is a simple example of signature verification.
	// In a real-world scenario, you would use a more secure method to verify the signature.
	valid := common.VerifySignature(challenge, p.secret, signature)
	if !valid {
		response.WriteHeader(http.StatusUnauthorized)
		return fmt.Errorf("invalid signature")
	}

	if !p.sess.IsValidSess(serialNumber, challenge) {
		response.WriteHeader(http.StatusBadRequest)
		return fmt.Errorf("invalid or expired session")
	}

	// mark the session as verified. only can verify once.
	if !p.sess.MarkSessVerified(serialNumber, challenge) {
		response.WriteHeader(http.StatusBadRequest)
		return fmt.Errorf("invalid or verified session")
	}

	response.Data = map[string]interface{}{
		"serial_number": serialNumber,
		"status":        "verified",
		"token":         p.sess.GenerateAuthHeader(serialNumber),
	}
	atomic.AddInt32(&p.count, 1)
	return nil
}

func (p *Plugin) Priority() int {
	return p.index
}
