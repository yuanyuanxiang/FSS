package firmware_update

// Package firmware_update provides a plugin for updating device firmware.
import (
	"context"
	"crypto/ecdh"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"github.com/luraproject/lura/v2/config"
	"github.com/luraproject/lura/v2/proxy"
	"github.com/luraproject/lura/v2/vicg"
	"github.com/yuanyuanxiang/fss/internal/pkg/common"
	"github.com/yuanyuanxiang/fss/pkg/audit"
)

type SessionManager interface {
	VerifyAuthHeader(authHeader string) (string, error)
}

type DeviceManager interface {
	IsDeviceRegistered(serialNumber string) error
	GetDevicePublicKey(serialNumber string) string
}

type factory struct {
	sess       SessionManager
	dev        DeviceManager
	serverPriv *ecdh.PrivateKey
}

// Plugin defines
type Plugin struct {
	factory
	name  string
	index int
	log   audit.LogManager
}

func NewFactory(sess SessionManager, dev DeviceManager, serverPriv *ecdh.PrivateKey) vicg.VicgPluginFactory {
	return factory{sess: sess, dev: dev, serverPriv: serverPriv}
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
	Deliver signed firmware update to authenticated devices

Header: <Authorization: "xxx">

Response:

	{
		"data": "base64 encrypted firmware data",
		"serial_number": "0000000001",
		"version": "1.0.1",
		"timestamp": 1234567890,
		"signature": "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
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
	// get firmware version
	version := request.Path[strings.LastIndex(request.Path, "/")+1:]
	if version == "" {
		response.WriteHeader(http.StatusBadRequest)
		p.log.AddUpdateLog(request.RemoteAddr, serialNumber, "request invalid version", http.StatusBadRequest)
		response.Data = map[string]interface{}{
			"code":          http.StatusBadRequest,
			"msg":           fmt.Sprintf("invalid version: %s", version),
			"serial_number": serialNumber,
		}
		return p.Error()
	}
	// check if device is already registered
	err = p.dev.IsDeviceRegistered(serialNumber)
	if err != nil {
		response.WriteHeader(http.StatusConflict)
		p.log.AddUpdateLog(request.RemoteAddr, serialNumber, "check device status failed", http.StatusConflict, err.Error())
		response.Data = map[string]interface{}{
			"code":          http.StatusConflict,
			"msg":           "check device status failed",
			"serial_number": serialNumber,
		}
		return p.Error()
	}
	// get client public key
	clientPubKey, err := common.Base64ToPublicKey(p.dev.GetDevicePublicKey(serialNumber))
	if err != nil {
		response.WriteHeader(http.StatusBadRequest)
		p.log.AddUpdateLog(request.RemoteAddr, serialNumber, "invalid public key", http.StatusBadRequest, err.Error())
		response.Data = map[string]interface{}{
			"code":          http.StatusBadRequest,
			"msg":           fmt.Sprintf("invalid public key: %v", err),
			"serial_number": serialNumber,
		}
		return p.Error()

	}
	// derive shared secret
	sharedSecret, _ := p.serverPriv.ECDH(clientPubKey)
	encKey, macKey := common.DeriveKeys(sharedSecret)
	// encrypt the firmware data
	var firmwareData = version
	encryptedData, err := common.EncryptData([]byte(firmwareData), encKey)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		p.log.AddUpdateLog(request.RemoteAddr, serialNumber, "failed to encrypt response", http.StatusInternalServerError, err.Error())
		response.Data = map[string]interface{}{
			"code":          http.StatusInternalServerError,
			"msg":           fmt.Sprintf("failed to encrypt response: %v", err),
			"serial_number": serialNumber,
		}
		return p.Error()
	}

	base64Data := base64.StdEncoding.EncodeToString(encryptedData)
	mac := common.SignSignature(base64Data, string(macKey))

	response.Data = map[string]interface{}{
		"code":          0,
		"msg":           "success",
		"serial_number": serialNumber,
		"data":          base64Data, // base64 encrypted firmware data
		"version":       version,
		"timestamp":     common.GetCurrentTimestamp(),
		"signature":     mac,
	}
	p.log.AddUpdateLog(request.RemoteAddr, serialNumber, "success", http.StatusOK)

	return nil
}

func (p *Plugin) Priority() int {
	return p.index
}

func (p *Plugin) Error() error {
	return fmt.Errorf("failed on plugin: '%s'", p.name)
}
