package server

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
	GetDeviceList() ([]map[string]interface{}, error)
	BlockDevice(serialNumber string) error
	AuthorizeDevice(serialNumber string) error
	IncreaseAllowance(key string, inc int) error
	GetAuditLogs(typ string) ([]map[string]interface{}, error)
}

func NewExecuter(addr string) Executer {
	return &ExecuterImpl{
		serverAddr: addr,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// Send http request to the server and display the results.
type ExecuterImpl struct {
	serverAddr string
	client     *http.Client
}

func (e *ExecuterImpl) request(method, url string, in map[string]interface{}) (map[string]interface{}, error) {
	var body io.Reader = http.NoBody
	if in != nil {
		data, _ := json.Marshal(in)
		body = bytes.NewBuffer(data)
	}
	req, err := http.NewRequest(method, fmt.Sprintf("http://%s%s", e.serverAddr, url), body)
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

func (e *ExecuterImpl) GetDeviceList() ([]map[string]interface{}, error) {
	ret, err := e.request(http.MethodGet, "/api/devices", nil)
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

func (e *ExecuterImpl) BlockDevice(serialNumber string) error {
	_, err := e.request(http.MethodPost, fmt.Sprintf("/api/devices/%s/block", serialNumber), nil)
	return err
}

func (e *ExecuterImpl) AuthorizeDevice(serialNumber string) error {
	_, err := e.request(http.MethodPost, fmt.Sprintf("/api/devices/%s/authorize", serialNumber), nil)
	return err
}

func (e *ExecuterImpl) IncreaseAllowance(key string, inc int) error {
	m := map[string]interface{}{
		"increase_allowance": inc,
	}
	_, err := e.request(http.MethodPost, "/api/update-allowance", m)
	return err
}

func (e *ExecuterImpl) GetAuditLogs(typ string) ([]map[string]interface{}, error) {
	ret, err := e.request(http.MethodGet, fmt.Sprintf("/api/logs/%s", typ), nil)
	if err != nil {
		return nil, err
	}
	arr, _ := ret["audit_logs"].([]interface{})
	out := make([]map[string]interface{}, len(arr))
	for i, a := range arr {
		out[i], _ = a.(map[string]interface{})
	}
	return out, nil
}
