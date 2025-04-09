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
	infra interface{}
}

func NewFactory(dev DeviceManager, serverPriv *ecdh.PrivateKey) vicg.VicgPluginFactory {
	return factory{dev: dev, serverPriv: serverPriv}
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
	version := request.Path[strings.LastIndex(request.Path, "/")+1:]
	if version == "" {
		response.WriteHeader(http.StatusBadRequest)
		return fmt.Errorf("invalid version: %s", version)
	}
	serialNumber := cvt.ToString(request.Private["serial_number"])
	// check if device is already registered
	if !p.dev.IsDeviceRegistered(serialNumber) {
		response.WriteHeader(http.StatusConflict)
		return fmt.Errorf("device already registered")
	}
	// get client public key
	clientPubKey, err := common.Base64ToPublicKey(p.dev.GetDevicePublicKey(serialNumber))
	if err != nil {
		response.WriteHeader(http.StatusBadRequest)
		return fmt.Errorf("invalid public key: %v", err)

	}
	// derive shared secret
	sharedSecret, _ := p.serverPriv.ECDH(clientPubKey)
	encKey, macKey := common.DeriveKeys(sharedSecret)
	// encrypt the firmware data
	var firmwareData = "1.0.1"
	encryptedData, err := common.EncryptData([]byte(firmwareData), encKey)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		return fmt.Errorf("failed to encrypt response: %v", err)
	}

	base64Data := base64.StdEncoding.EncodeToString(encryptedData)
	mac := common.SignSignature(base64Data, string(macKey))

	response.Data = map[string]interface{}{
		"serial_number": serialNumber,
		"data":          base64Data, // base64 encrypted firmware data
		"version":       version,
		"timestamp":     common.GetCurrentTimestamp(),
		"signature":     mac,
	}

	return nil
}

func (p *Plugin) Priority() int {
	return p.index
}
