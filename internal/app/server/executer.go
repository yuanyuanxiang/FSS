package server

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	cvt "github.com/yuanyuanxiang/fss/pkg/convert"
)

type Executer interface {
	GetDeviceList() ([]map[string]interface{}, error)
	BlockDevice(serialNumber string) error
	AuthorizeDevice(serialNumber string) error
	IncreaseAllowance(key string, inc int) (int, error)
	GetAuditLogs(typ string) ([]map[string]interface{}, error)
}

func NewExecuter(addr string, opts ...Option) (Executer, error) {
	exe := &ExecuterImpl{
		protocol:   "http",
		serverAddr: addr,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
	for _, f := range opts {
		if err := f(exe); err != nil {
			return nil, err
		}
	}
	return exe, nil
}

// Send http request to the server and display the results.
type ExecuterImpl struct {
	protocol   string
	serverAddr string
	client     *http.Client
}

type Option func(*ExecuterImpl) error

func WithCertFile(certFile string) Option {
	return func(e *ExecuterImpl) error {
		caCert, err := ioutil.ReadFile(certFile)
		if err != nil {
			return fmt.Errorf("failed to read CA certificate: %v", err)
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)
		e.protocol = "https"
		e.client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: caCertPool,
			},
		}
		return nil
	}
}

func (e *ExecuterImpl) request(method, url string, in map[string]interface{}) (map[string]interface{}, error) {
	var body io.Reader = http.NoBody
	if in != nil {
		data, _ := json.Marshal(in)
		body = bytes.NewBuffer(data)
	}
	req, err := http.NewRequest(method, fmt.Sprintf("%s://%s%s", e.protocol, e.serverAddr, url), body)
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

func (e *ExecuterImpl) IncreaseAllowance(key string, inc int) (int, error) {
	m := map[string]interface{}{
		"increase_allowance": inc,
	}
	v, err := e.request(http.MethodPost, "/api/update-allowance", m)
	return cvt.ToInt(v["allowance"]), err
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
