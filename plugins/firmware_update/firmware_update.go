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
	cvt "github.com/yuanyuanxiang/fss/pkg/convert"
)

type DeviceManager interface {
	IsDeviceRegistered(serialNumber string) bool
	GetDevicePublicKey(serialNumber string) string
}

type factory struct {
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

func NewFactory(dev DeviceManager, serverPriv *ecdh.PrivateKey) vicg.VicgPluginFactory {
	return factory{dev: dev, serverPriv: serverPriv}
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

Request:

	{
		"serial_number": "0000000001",
		"signature": "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		"challenge": "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	}

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
	serialNumber := cvt.ToString(request.Private["serial_number"])
	version := request.Path[strings.LastIndex(request.Path, "/")+1:]
	if version == "" {
		response.WriteHeader(http.StatusBadRequest)
		p.log.AddUpdateLog(request.RemoteAddr, serialNumber, "request invalid version", http.StatusBadRequest)
		return fmt.Errorf("invalid version: %s", version)
	}
	// check if device is already registered
	if !p.dev.IsDeviceRegistered(serialNumber) {
		response.WriteHeader(http.StatusConflict)
		p.log.AddUpdateLog(request.RemoteAddr, serialNumber, "device not registered", http.StatusConflict)
		return fmt.Errorf("device not registered")
	}
	// get client public key
	clientPubKey, err := common.Base64ToPublicKey(p.dev.GetDevicePublicKey(serialNumber))
	if err != nil {
		response.WriteHeader(http.StatusBadRequest)
		p.log.AddUpdateLog(request.RemoteAddr, serialNumber, "invalid public key", http.StatusBadRequest, err)
		return fmt.Errorf("invalid public key: %v", err)

	}
	// derive shared secret
	sharedSecret, _ := p.serverPriv.ECDH(clientPubKey)
	encKey, macKey := common.DeriveKeys(sharedSecret)
	// encrypt the firmware data
	var firmwareData = version
	encryptedData, err := common.EncryptData([]byte(firmwareData), encKey)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		p.log.AddUpdateLog(request.RemoteAddr, serialNumber, "failed to encrypt response", http.StatusInternalServerError, err)
		return fmt.Errorf("failed to encrypt response: %v", err)
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
