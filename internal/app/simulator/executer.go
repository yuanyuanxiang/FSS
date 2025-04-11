package simulator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	cvt "github.com/yuanyuanxiang/fss/pkg/convert"
)

type Executer interface {
	GenerateDevices(master string, count int, startSerial int) error
	UpdateDevice(serialNumber int, version string) error
	BatchUpdate(startSerial, endSerial int, version string) error
	GetDeviceStatus(serialNumber int) (map[string]interface{}, error)
	GetDeviceList() ([]map[string]interface{}, error)
	Replay(serialNumber int) error
	BatchReplay(startSerial, endSerial int) error
}

type ExecuterImpl struct {
	simulatorAddr string
	client        *http.Client
}

func NewExecuter(addr string) Executer {
	return &ExecuterImpl{
		simulatorAddr: addr,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (e *ExecuterImpl) request(method, url string, in map[string]interface{}) (map[string]interface{}, error) {
	var body io.Reader = http.NoBody
	if in != nil {
		data, _ := json.Marshal(in)
		body = bytes.NewBuffer(data)
	}
	req, err := http.NewRequest(method, fmt.Sprintf("http://%s%s", e.simulatorAddr, url), body)
	if err != nil {
		return nil, err
	}
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var out map[string]interface{}
	err = json.Unmarshal(data, &out)
	if err != nil {
		return nil, err
	}
	if code := cvt.ToInt(out["code"]); code != 0 {
		return out, fmt.Errorf("%s", cvt.ToString(out["msg"]))
	}
	return out, nil
}

func (e *ExecuterImpl) GenerateDevices(master string, count int, startSerial int) error {
	m := map[string]interface{}{
		"master_address": master,
		"generate":       count,
		"start-serial":   startSerial,
	}
	_, err := e.request(http.MethodPost, "/api/generate", m)
	return err
}

func (e *ExecuterImpl) UpdateDevice(serialNumber int, version string) error {
	m := map[string]interface{}{
		"version": version,
	}
	_, err := e.request(http.MethodPost, fmt.Sprintf("/api/devices/%d/request-update", serialNumber), m)
	return err
}

func (e *ExecuterImpl) BatchUpdate(startSerial, endSerial int, version string) error {
	m := map[string]interface{}{
		"start_serial": startSerial,
		"end_serial":   endSerial,
		"version":      version,
	}
	_, err := e.request(http.MethodPost, "/api/devices/batch-update", m)
	return err
}

func (e *ExecuterImpl) GetDeviceStatus(serialNumber int) (map[string]interface{}, error) {
	return e.request(http.MethodGet, fmt.Sprintf("/api/devices/%v", serialNumber), nil)
}

func (e *ExecuterImpl) GetDeviceList() ([]map[string]interface{}, error) {
	ret, err := e.request(http.MethodGet, "/api/devices/status", nil)
	if err != nil {
		return nil, err
	}
	arr, _ := ret["devices"].([]interface{})
	out := make([]map[string]interface{}, len(arr))
	for i, a := range arr {
		out[i], _ = a.(map[string]interface{})
	}
	return out, nil
}

func (e *ExecuterImpl) Replay(serialNumber int) error {
	_, err := e.request(http.MethodPost, fmt.Sprintf("/api/simulate/replay/%v", serialNumber), nil)
	return err
}

func (e *ExecuterImpl) BatchReplay(startSerial, endSerial int) error {
	m := map[string]interface{}{
		"end_serial": endSerial,
	}
	_, err := e.request(http.MethodPost, fmt.Sprintf("/api/simulate/replay/%v", startSerial), m)
	return err
}
