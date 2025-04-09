package simulator

import (
	"encoding/json"
	"testing"

	"github.com/yuanyuanxiang/fss/internal/pkg/common"
)

func TestDevice_MarshalJSON(t *testing.T) {
	// Create a device instance
	device, err := NewDevice("127.0.0.1:9000", 123456, "1.0.0", common.SymmetricKey)
	if err != nil {
		log.Fatalf("Error creating or loading device: %v", err)
	}

	// Marshal the device to JSON
	jsonData, err := json.Marshal(device)
	if err != nil {
		t.Fatalf("Failed to marshal device: %v", err)
	}

	// Unmarshal the JSON back to a Device struct
	var unmarshaledDevice Device
	err = json.Unmarshal(jsonData, &unmarshaledDevice)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// Check if the unmarshaled data matches the original device
	if device.SerialNumber != unmarshaledDevice.SerialNumber {
		t.Errorf("Expected SerialNumber %s, got %s", device.SerialNumber, unmarshaledDevice.SerialNumber)
	}
	if device.FirmwareVersion != unmarshaledDevice.FirmwareVersion {
		t.Errorf("Expected FirmwareVersion %s, got %s", device.FirmwareVersion, unmarshaledDevice.FirmwareVersion)
	}
	if device.State != unmarshaledDevice.State {
		t.Errorf("Expected State %s, got %s", device.State, unmarshaledDevice.State)
	}
	if string(device.SymmetricKey) != string(unmarshaledDevice.SymmetricKey) {
		t.Errorf("Expected SymmetricKey %s, got %s", string(device.SymmetricKey), string(unmarshaledDevice.SymmetricKey))
	}
}
