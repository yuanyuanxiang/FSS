package server

// Implementation of the DeviceManager and SessionManager interfaces
// for managing device registration and session management in the server.

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/yuanyuanxiang/fss/internal/pkg/common"
	cvt "github.com/yuanyuanxiang/fss/pkg/convert"
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
