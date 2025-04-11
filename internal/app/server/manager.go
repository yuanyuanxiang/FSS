package server

// Implementation of the DeviceManager and SessionManager interfaces
// for managing device registration and session management in the server.

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/yuanyuanxiang/fss/internal/pkg/common"
	cvt "github.com/yuanyuanxiang/fss/pkg/convert"
)

const (
	CONFIG_PATH = "settings.ini"
)

// SessionManager interface defines methods for managing user sessions.
type SessionManager interface {
	AddSess(serialNumber, challenge string, expiresAt time.Time, isVerified bool)
	IsValidSess(serialNumber, challenge string) bool
	MarkSessVerified(serialNumber, challenge string) bool

	GenerateAuthHeader(serialNumber string) string
	VerifyAuthHeader(authHeader string) (string, error)
}

type Session struct {
	SerialNumber string
	Challenge    string
	ExpiresAt    time.Time
	IsVerified   bool
}

type SessionManagerImpl struct {
	mu       sync.Mutex
	Sessions map[string]Session
	Tokkens  map[string]struct{} // one time token
}

func NewSessionManager() *SessionManagerImpl {
	return &SessionManagerImpl{
		Sessions: make(map[string]Session),
		Tokkens:  make(map[string]struct{}),
	}
}

func (s *SessionManagerImpl) AddSess(serialNumber, challenge string, expiresAt time.Time, isVerified bool) {
	sessId := fmt.Sprintf("%s-%s", serialNumber, challenge)
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.Sessions[sessId]; !ok {
		s.Sessions[sessId] = Session{
			SerialNumber: serialNumber,
			Challenge:    challenge,
			ExpiresAt:    expiresAt,
			IsVerified:   isVerified,
		}
	}
}

func (s *SessionManagerImpl) IsValidSess(serialNumber, challenge string) bool {
	sessId := fmt.Sprintf("%s-%s", serialNumber, challenge)
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess, ok := s.Sessions[sessId]; ok {
		if time.Now().After(sess.ExpiresAt) {
			delete(s.Sessions, sessId)
			return false
		}
		return true
	}
	return false
}

func (s *SessionManagerImpl) MarkSessVerified(serialNumber, challenge string) bool {
	sessId := fmt.Sprintf("%s-%s", serialNumber, challenge)
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess, ok := s.Sessions[sessId]; ok {
		if time.Now().After(sess.ExpiresAt) {
			delete(s.Sessions, sessId)
			return false
		}
		if sess.IsVerified {
			return false
		}
		sess.IsVerified = true
		return true
	}
	return false
}

func (s *SessionManagerImpl) GenerateAuthHeader(serialNumber string) string {
	// finally we should use JWT or other token
	// length: 7 + 10 + 15 = 32
	str, _ := common.GenerateRandomStringBase64(15)
	token := "Bearer " + serialNumber + str
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Tokkens[token] = struct{}{}
	return token
}

func (s *SessionManagerImpl) VerifyAuthHeader(authHeader string) (string, error) {
	if !strings.HasPrefix(authHeader, "Bearer ") || len(authHeader) < 32 {
		return "", fmt.Errorf("invalid auth header")
	}
	str := strings.TrimPrefix(authHeader, "Bearer ")
	serialNumber := str[:10]
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.Tokkens[authHeader]; !ok {
		return "", fmt.Errorf("invalid auth header")
	}
	delete(s.Tokkens, authHeader)
	return serialNumber, nil
}

///////////////////////////////////////////////////////////////////////////////////////////////////////

// DeviceManager interface defines methods for managing device registration and verification.
type DeviceManager interface {
	IsDeviceRegistered(serialNumber string) error
	RegisterDevice(serialNumber, publicKey string, isVerified bool)
	GetDevicePublicKey(serialNumber string) string
	GetDeviceList() ([]map[string]interface{}, error)
	BlockDevice(serialNumber string) error
	AuthorizeDevice(serialNumber string) error

	GetAllowance(key string) int
	IncreaseAllowance(key string, inc int)
}

type DeviceManagerImpl struct {
	mu        sync.Mutex
	Allowance int
	devList   map[string]map[string]interface{}
}

func NewDeviceManager(allowance int) *DeviceManagerImpl {
	dev := &DeviceManagerImpl{
		Allowance: allowance,
		devList:   make(map[string]map[string]interface{}),
	}
	data, _ := os.ReadFile(CONFIG_PATH)
	settings := map[string]interface{}{}
	err := json.Unmarshal(data, &settings)
	if err == nil {
		dev.Allowance = cvt.ToInt(settings["allowance"])
	}
	return dev
}

func (d *DeviceManagerImpl) IsDeviceRegistered(serialNumber string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if m, ok := d.devList[serialNumber]; ok {
		authorize := cvt.ToBoolean(m["is_verified"])
		if !authorize {
			return fmt.Errorf("device is authorized")
		}
		return nil
	}
	return fmt.Errorf("device not registered")
}

func (d *DeviceManagerImpl) RegisterDevice(serialNumber, publicKey, state string, isVerified bool) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.Allowance <= 0 {
		return fmt.Errorf("allowance exceeded")
	}
	if _, ok := d.devList[serialNumber]; !ok {
		d.Allowance--
	}
	d.devList[serialNumber] = make(map[string]interface{})
	d.devList[serialNumber]["serial_number"] = serialNumber
	d.devList[serialNumber]["public_key"] = publicKey
	d.devList[serialNumber]["is_verified"] = isVerified
	d.devList[serialNumber]["state"] = state

	return nil
}

func (d *DeviceManagerImpl) GetDevicePublicKey(serialNumber string) string {
	d.mu.Lock()
	defer d.mu.Unlock()
	if dev, ok := d.devList[serialNumber]; ok {
		return cvt.ToString(dev["public_key"])
	}
	return ""
}

func (d *DeviceManagerImpl) GetDeviceList() ([]map[string]interface{}, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	devices := make([]map[string]interface{}, 0, len(d.devList))
	for _, device := range d.devList {
		devices = append(devices, device)
	}
	return devices, nil
}

func (d *DeviceManagerImpl) BlockDevice(serialNumber string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if m, ok := d.devList[serialNumber]; ok {
		m["is_verified"] = false // marked as unauthorized
	} else {
		d.devList[serialNumber] = make(map[string]interface{})
		d.devList[serialNumber]["serial_number"] = serialNumber
		d.devList[serialNumber]["public_key"] = ""
		d.devList[serialNumber]["is_verified"] = false
		d.devList[serialNumber]["state"] = ""
	}
	return nil
}

func (d *DeviceManagerImpl) AuthorizeDevice(serialNumber string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if m, ok := d.devList[serialNumber]; ok {
		m["is_verified"] = true // marked as unauthorized
	} else {
		d.devList[serialNumber] = make(map[string]interface{})
		d.devList[serialNumber]["serial_number"] = serialNumber
		d.devList[serialNumber]["public_key"] = ""
		d.devList[serialNumber]["is_verified"] = true
		d.devList[serialNumber]["state"] = ""
	}
	return nil
}

func (d *DeviceManagerImpl) GetAllowance(key string) int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.Allowance
}

func (d *DeviceManagerImpl) IncreaseAllowance(key string, inc int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Allowance += inc
	// use database instead
	data, _ := json.MarshalIndent(map[string]interface{}{"allowance": d.Allowance}, "", "  ")
	_ = os.WriteFile(CONFIG_PATH, data, 0644)
}
