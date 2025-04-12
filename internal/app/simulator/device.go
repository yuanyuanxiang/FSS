package simulator

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/yuanyuanxiang/fss/internal/pkg/common"
	cvt "github.com/yuanyuanxiang/fss/pkg/convert"
	"github.com/yuanyuanxiang/fss/pkg/logger"
)

var log, _ = logger.NewLogger()

var client = &http.Client{
	Timeout: 15 * time.Second,
}

// DeviceState represents the device state
type DeviceState string

const (
	Bootloader DeviceState = "bootloader"
	Updated    DeviceState = "updated"
)

// UpdateRecord represents an update history record
type UpdateRecord struct {
	Version   string    `json:"version"`
	Timestamp time.Time `json:"timestamp"`
}

// Device represents the device with various fields including keys
type Device struct {
	ServerPublicKey *ecdh.PublicKey  `json:"server_pubkey"`  // Server public key for encryption
	MasterAddress   string           `json:"master_address"` // Master address of the device
	SerialNumber    string           `json:"serial_number"`
	FirmwareVersion string           `json:"firmware_version"`
	State           DeviceState      `json:"state"`
	SymmetricKey    []byte           `json:"symmetric_key"`
	PrivateKey      *ecdh.PrivateKey `json:"private_key,omitempty"`
	PublicKey       *ecdh.PublicKey  `json:"public_key,omitempty"`
	UpdateHistory   []UpdateRecord   `json:"update_history"`
}

type Callback func(d *Device, v map[string]interface{}, auth, version string) error

func (d *Device) GetChallenge() (string, error) {
	// get challenge
	resp, err := client.Get(fmt.Sprintf("http://%s/api/challenge/%s", d.MasterAddress, d.SerialNumber))
	if err != nil {
		return "", err
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var m map[string]interface{}
	err = json.Unmarshal(data, &m)
	if err != nil {
		return "", err
	}
	serial := cvt.ToString(m["serial_number"])
	if serial != d.SerialNumber {
		return "", fmt.Errorf("serial number mismatch: expected %s, got %s", d.SerialNumber, serial)
	}
	challenge := cvt.ToString(m["challenge"])
	return challenge, nil
}

func (d *Device) GetToken(challenge string) (string, error) {
	signature := common.SignSignature(challenge, string(d.SymmetricKey))
	// verify
	data, _ := json.Marshal(map[string]interface{}{"serial_number": d.SerialNumber, "signature": signature, "challenge": challenge})
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://%s/api/verify", d.MasterAddress), bytes.NewBuffer(data))
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("failed to verify device: %s", resp.Status)
	}
	data, _ = io.ReadAll(resp.Body)
	var token map[string]interface{}
	err = json.Unmarshal(data, &token)
	if err != nil {
		return "", err
	}
	auth := cvt.ToString(token["token"])
	return auth, nil
}

func (d *Device) RegisterProc(ctx context.Context, duration time.Duration) {
	ticker := time.NewTicker(duration)
	registered := false
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if registered {
				continue
			}
			if err := d.Register(); err != nil {
				continue
			}
			registered = true
		}
	}
}

func (d *Device) Register() error {
	// get challenge
	challenge, err := d.GetChallenge()
	if err != nil {
		return err
	}
	// verify
	auth, err := d.GetToken(challenge)
	if err != nil {
		return err
	}
	// register
	pubKeyBase64 := common.PublicKeyToBase64(d.PublicKey)
	data, _ := json.Marshal(map[string]interface{}{"serial_number": d.SerialNumber, "public_key": pubKeyBase64, "state": d.State})
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://%s/api/register", d.MasterAddress), bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", auth)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to register device: %s", resp.Status)
	}
	data, _ = io.ReadAll(resp.Body)
	var m map[string]interface{}
	err = json.Unmarshal(data, &m)
	if err != nil {
		return err
	}
	// save the server public key
	publicKey := cvt.ToString(m["public_key"])
	d.ServerPublicKey, err = common.Base64ToPublicKey(publicKey)
	if err != nil {
		return err
	}
	log.Infof("Device %s registered to '%s' succeed\n", d.SerialNumber, d.MasterAddress)
	return d.Save()
}

func (d *Device) Update(callback Callback, version string) error {
	if d.ServerPublicKey == nil {
		return fmt.Errorf("server public key is nil")
	}
	// get challenge
	challenge, err := d.GetChallenge()
	if err != nil {
		return err
	}
	// verify
	auth, err := d.GetToken(challenge)
	if err != nil {
		return err
	}

	signature := common.SignSignature(challenge, string(d.SymmetricKey))
	// update the device
	return callback(d, map[string]interface{}{
		"serial_number": d.SerialNumber,
		"challenge":     challenge,
		"signature":     signature,
	}, auth, version)
}

// communicate with server to get firmware
func getFirmware(d *Device, v map[string]interface{}, auth, version string) error {
	data, _ := json.Marshal(v)
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://%s/api/firmware/%s", d.MasterAddress, version),
		bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", auth)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to register device: %s", resp.Status)
	}
	data, err = io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var m map[string]interface{}
	err = json.Unmarshal(data, &m)
	if err != nil {
		return err
	}
	code := cvt.ToInt(m["code"])
	if code != 0 {
		return fmt.Errorf("failed to update device: %d[%v]", code, m["msg"])
	}
	base64Data := cvt.ToString(m["data"])
	data, err = base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return fmt.Errorf("failed to decode base64 data: %v", err)
	}
	mac := cvt.ToString(m["signature"])
	// derive shared secret
	sharedSecret, _ := d.PrivateKey.ECDH(d.ServerPublicKey)
	encKey, macKey := common.DeriveKeys(sharedSecret)
	// check signature
	if !common.VerifySignature(base64Data, string(macKey), mac) {
		return fmt.Errorf("failed to verify signature")
	}
	// decrypt data
	firmwareData, err := common.DecryptData(data, encKey)
	if err != nil {
		return fmt.Errorf("failed to decrypt firmware: %v", err)
	}
	// mark device as updated
	d.State = Updated
	d.UpdateHistory = append(d.UpdateHistory, UpdateRecord{
		Version:   version,
		Timestamp: time.Now(),
	})
	// save firmware data to file
	d.FirmwareVersion = version
	log.Printf("Device %s updated to firmware version %s\n", d.SerialNumber, string(firmwareData))

	return d.Save()
}

// MarshalJSON customizes the JSON marshaling for Device
func (d *Device) MarshalJSON() ([]byte, error) {
	type Alias Device // Create an alias to avoid recursion in the Marshal method
	var pubKeyBase64, privKeyBase64, svrPubkey string

	// Marshal public key as base64-encoded string
	if d.PublicKey != nil {
		pubKeyBase64 = common.PublicKeyToBase64(d.PublicKey)
	}

	// Marshal private key as base64-encoded string
	if d.PrivateKey != nil {
		privKeyBase64 = common.PrivateKeyToBase64(d.PrivateKey)
	}
	if d.ServerPublicKey != nil {
		svrPubkey = common.PublicKeyToBase64(d.ServerPublicKey)
	}
	// Return the struct with the keys encoded as strings
	return json.Marshal(&struct {
		*Alias
		PrivateKey      string `json:"private_key,omitempty"`
		PublicKey       string `json:"public_key,omitempty"`
		ServerPublicKey string `json:"server_pubkey,omitempty"`
	}{
		Alias:           (*Alias)(d),
		PrivateKey:      privKeyBase64,
		PublicKey:       pubKeyBase64,
		ServerPublicKey: svrPubkey,
	})
}

// UnmarshalJSON customizes the JSON unmarshaling for Device
func (d *Device) UnmarshalJSON(data []byte) error {
	type Alias Device // Create an alias to avoid recursion in the Unmarshal method
	aux := &struct {
		*Alias
		PrivateKey      string `json:"private_key,omitempty"`
		PublicKey       string `json:"public_key,omitempty"`
		ServerPublicKey string `json:"server_pubkey,omitempty"`
	}{
		Alias: (*Alias)(d),
	}

	// Unmarshal the base structure
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Decode private key if available
	if aux.PrivateKey != "" {
		privKey, err := common.Base64ToPrivateKey(aux.PrivateKey)
		if err != nil {
			return fmt.Errorf("error decoding private key: %v", err)
		}
		// Set the private key
		d.PrivateKey = privKey
	}

	// Decode public key if available
	if aux.PublicKey != "" {
		pubKey, err := common.Base64ToPublicKey(aux.PublicKey)
		if err != nil {
			return fmt.Errorf("error decoding public key: %v", err)
		}
		// Set the public key
		d.PublicKey = pubKey
	}

	if aux.ServerPublicKey != "" {
		pubKey, err := common.Base64ToPublicKey(aux.ServerPublicKey)
		if err != nil {
			return fmt.Errorf("error decoding server public key: %v", err)
		}
		d.ServerPublicKey = pubKey
	}

	return nil
}

func (d *Device) Save() error {
	fileName := fmt.Sprintf("%s.json", d.SerialNumber)
	// Marshal the device to JSON
	deviceJSON, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling device to JSON: %v", err)
	}
	// Save the new device to the file
	if err := os.WriteFile(fileName, deviceJSON, 0644); err != nil {
		return fmt.Errorf("error writing device file: %v", err)
	}

	log.Printf("Device %s created and saved to file", d.SerialNumber)
	return nil
}

// NewDevice creates a new device if the device doesn't exist. If it exists, it loads from JSON.
func NewDevice(master string, serial int, firmwareVersion, symmetricKey string) (*Device, error) {
	serialNumber := fmt.Sprintf("%010d", serial)
	fileName := fmt.Sprintf("%s.json", serialNumber)

	// Check if the file exists
	if _, err := os.Stat(fileName); err == nil {
		// File exists, load the device from JSON
		log.Printf("Device %s found, loading from JSON", serialNumber)

		// Read the file
		data, err := os.ReadFile(fileName)
		if err != nil {
			return nil, fmt.Errorf("error reading device file: %v", err)
		}

		// Unmarshal the device data
		var device Device
		if err := json.Unmarshal(data, &device); err != nil {
			return nil, fmt.Errorf("error unmarshaling device data: %v", err)
		}

		return &device, nil
	} else if os.IsNotExist(err) {
		// File does not exist, create a new device
		log.Printf("Device %s not found, creating a new device", serialNumber)

		// Create a new device
		priv, err := ecdh.P384().GenerateKey(rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("error generating keys: %v", err)
		}

		// Create the new device object
		device := &Device{
			MasterAddress:   master,
			SerialNumber:    serialNumber,
			FirmwareVersion: firmwareVersion,
			State:           Bootloader,
			SymmetricKey:    []byte(symmetricKey),
			PrivateKey:      priv,
			PublicKey:       priv.PublicKey(),
			UpdateHistory:   []UpdateRecord{},
		}

		// Marshal the device to JSON
		deviceJSON, err := json.MarshalIndent(device, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("error marshaling device to JSON: %v", err)
		}

		// Save the new device to the file
		if err := os.WriteFile(fileName, deviceJSON, 0644); err != nil {
			return nil, fmt.Errorf("error writing device file: %v", err)
		}

		log.Printf("Device %s created and saved to file", serialNumber)

		return device, nil
	} else {
		// Some other error occurred while checking file existence
		return nil, fmt.Errorf("error checking if file exists: %v", err)
	}
}
