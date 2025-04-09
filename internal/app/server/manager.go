package server

// Implementation of the DeviceManager and SessionManager interfaces
// for managing device registration and session management in the server.

import (
	"fmt"
	"strings"
	"sync"
	"time"

	cvt "github.com/yuanyuanxiang/fss/pkg/convert"
)

// SessionManager interface defines methods for managing user sessions.
type SessionManager interface {
	AddSess(serialNumber, challenge string, expiresAt time.Time, isVerified bool)
	IsValidSess(serialNumber string) bool
	IsVerifiedSess(serialNumber string) bool
	MarkSessVerified(serialNumber string)
	GenerateAuthHeader(serialNumber string) string
	VerifyAuthHeader(authHeader string) (string, error)
}

type SessionManagerImpl struct {
}

func NewSessionManager() *SessionManagerImpl {
	return &SessionManagerImpl{}
}

func (s *SessionManagerImpl) AddSess(serialNumber, challenge string, expiresAt time.Time, isVerified bool) {

}

func (s *SessionManagerImpl) IsValidSess(serialNumber string) bool {
	return true
}

func (s *SessionManagerImpl) IsVerifiedSess(serialNumber string) bool {
	return true
}

func (s *SessionManagerImpl) MarkSessVerified(serialNumber string) {

}

func (s *SessionManagerImpl) GenerateAuthHeader(serialNumber string) string {
	// finally we should use JWT or other token
	return "Bearer " + serialNumber
}
func (s *SessionManagerImpl) VerifyAuthHeader(authHeader string) (string, error) {
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer "), nil
	}
	return "", fmt.Errorf("invalid auth header")
}

///////////////////////////////////////////////////////////////////////////////////////////////////////

// DeviceManager interface defines methods for managing device registration and verification.
type DeviceManager interface {
	IsDeviceRegistered(serialNumber string) bool
	RegisterDevice(serialNumber, publicKey string, isVerified bool)
	GetDevicePublicKey(serialNumber string) string
	GetDeviceList() ([]map[string]interface{}, error)
	BlockDevice(serialNumber string) error
	AuthorizeDevice(serialNumber string) error
}

type DeviceManagerImpl struct {
	mu      sync.Mutex
	DevList map[string]map[string]interface{}
}

func NewDeviceManager() *DeviceManagerImpl {
	return &DeviceManagerImpl{
		DevList: make(map[string]map[string]interface{}),
	}
}

func (d *DeviceManagerImpl) IsDeviceRegistered(serialNumber string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, ok := d.DevList[serialNumber]; ok {
		return true
	}
	return false
}

func (d *DeviceManagerImpl) RegisterDevice(serialNumber, publicKey string, isVerified bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, ok := d.DevList[serialNumber]; !ok {
		d.DevList[serialNumber] = make(map[string]interface{})
	}
	d.DevList[serialNumber]["public_key"] = publicKey
	d.DevList[serialNumber]["is_verified"] = isVerified
	fmt.Printf("Device %s registered with public key: %s\n", serialNumber, publicKey)
}

func (d *DeviceManagerImpl) GetDevicePublicKey(serialNumber string) string {
	d.mu.Lock()
	defer d.mu.Unlock()
	if dev, ok := d.DevList[serialNumber]; ok {
		return cvt.ToString(dev["public_key"])
	}
	return ""
}

func (d *DeviceManagerImpl) GetDeviceList() ([]map[string]interface{}, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	devices := make([]map[string]interface{}, 0, len(d.DevList))
	for _, device := range d.DevList {
		devices = append(devices, device)
	}
	return devices, nil
}

func (d *DeviceManagerImpl) BlockDevice(serialNumber string) error {
	return nil
}

func (d *DeviceManagerImpl) AuthorizeDevice(serialNumber string) error {
	return nil
}
