package simulator

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-contrib/pprof"
	"github.com/luraproject/lura/v2/config"
	"github.com/luraproject/lura/v2/logging"
	"github.com/luraproject/lura/v2/router/gin"
	"github.com/luraproject/lura/v2/vicg"
	"github.com/yuanyuanxiang/fss/internal/pkg/common"
	"github.com/yuanyuanxiang/fss/pkg/audit"
	"github.com/yuanyuanxiang/fss/pkg/logger"
	"github.com/yuanyuanxiang/fss/plugins/batch_update"
	"github.com/yuanyuanxiang/fss/plugins/device_list"
	"github.com/yuanyuanxiang/fss/plugins/device_simulate"
	"github.com/yuanyuanxiang/fss/plugins/device_status"
	"github.com/yuanyuanxiang/fss/plugins/httpdata_parse"
	"github.com/yuanyuanxiang/fss/plugins/replay_simulate"
	"github.com/yuanyuanxiang/fss/plugins/request_update"
)

const (
	InitialVersion = "1.0.0"
)

// Simulator application
type Simulator struct {
	ctx      context.Context
	mu       sync.Mutex
	name     string // Module name
	cfg      string // Configuration file path
	log      logger.Logger
	devices  []*Device
	port     int
	ready    bool
	protocol string
	client   *http.Client
}

type Option func(*Simulator) error

func New(log logger.Logger, opts ...Option) *Simulator {
	sim := &Simulator{
		log:  log,
		name: "simulator",
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
	for _, f := range opts {
		if err := f(sim); err != nil {
			log.Errorf("Failed to apply option: %v", err)
		}
	}
	return sim
}

func WithCertFile(certFile string) Option {
	return func(sim *Simulator) error {
		caCert, err := ioutil.ReadFile(certFile)
		if err != nil {
			return fmt.Errorf("failed to read CA certificate: %v", err)
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)
		sim.protocol = "https"
		sim.client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: caCertPool,
			},
		}
		return nil
	}
}

func (sim *Simulator) GetName() string {
	return sim.name
}

// Generate specified number of devices
// startSerial is the starting serial number for device generation
// mimimum value is 0
// master is the master address for device registration
// Note: if the device already exists, it will do nothing
// After the device is generated, it will be registered to the master
// The device will be registered in a separate goroutine
func (sim *Simulator) GenerateDevices(master string, count int, startSerial int) error {
	sim.mu.Lock()
	defer sim.mu.Unlock()
	for i := 0; i < count; i++ {
		id := i + startSerial
		if sim.IsDeviceExit(id) {
			sim.log.Infof("Device '%v' is already exist\n", id)
			continue
		}
		device, err := NewDevice(master, id, InitialVersion, common.SymmetricKey)
		if err != nil {
			sim.log.Printf("Failed to generate device %v: %v\n", id, err)
			continue
		}
		sim.devices = append(sim.devices, device.SetSimulator(sim))
		go device.RegisterProc(sim.ctx, 5*time.Second)
		sim.log.Printf("Generated device: SerialNumber=%v\n", id)
	}
	return nil
}

func (sim *Simulator) IsDeviceExit(serialNumber int) bool {
	var id = fmt.Sprintf("%010d", serialNumber)
	for _, d := range sim.devices {
		if d.SerialNumber == id {
			return true
		}
	}
	return false
}

// Request update for a specific device
// Update the device firmware to the specified version
// If the device is not found, an error will be returned
func (sim *Simulator) UpdateDevice(serialNumber int, version string) error {
	serialNumberStr := fmt.Sprintf("%010d", serialNumber)
	sim.mu.Lock()
	defer sim.mu.Unlock()
	for _, device := range sim.devices {
		if device.SerialNumber == serialNumberStr {
			return device.Update(getFirmware, version)
		}
	}
	sim.log.Printf("Device with serial number %v not found.\n", serialNumber)
	return fmt.Errorf("device not found")
}

// Request updates for a range of devices
func (sim *Simulator) BatchUpdate(startSerial, endSerial int, version string) error {
	for i := startSerial; i <= endSerial; i++ {
		_ = sim.UpdateDevice(i, version)
	}
	return nil
}

// Show the status of a specific device
func (sim *Simulator) GetDeviceStatus(serialNumber int) (map[string]interface{}, error) {
	serialNumberStr := fmt.Sprintf("%010d", serialNumber)
	sim.mu.Lock()
	defer sim.mu.Unlock()
	for _, device := range sim.devices {
		if device.SerialNumber == serialNumberStr {
			sim.log.Printf("Device %v: FirmwareVersion=%s, State=%s\n", serialNumber, device.FirmwareVersion, device.State)
			return map[string]interface{}{
				"serial_number":    serialNumberStr,
				"firmware_version": device.FirmwareVersion,
				"state":            device.State,
				"update_history":   device.UpdateHistory,
			}, nil
		}
	}
	sim.log.Printf("Device with serial number %v not found.\n", serialNumber)
	return nil, fmt.Errorf("device not found")
}

// List all devices with their status
func (sim *Simulator) GetDeviceList() ([]map[string]interface{}, error) {
	var deviceList []map[string]interface{}
	sim.mu.Lock()
	defer sim.mu.Unlock()
	for _, device := range sim.devices {
		sim.log.Printf("Device %s: FirmwareVersion=%s, State=%s\n", device.SerialNumber, device.FirmwareVersion, device.State)
		deviceList = append(deviceList, map[string]interface{}{
			"serial_number":    device.SerialNumber,
			"firmware_version": device.FirmwareVersion,
			"state":            device.State,
			"update_history":   device.UpdateHistory,
		})
	}
	return deviceList, nil
}

func (sim *Simulator) GetDevice(serialNumber int) *Device {
	serialNumberStr := fmt.Sprintf("%010d", serialNumber)
	sim.mu.Lock()
	defer sim.mu.Unlock()
	for _, device := range sim.devices {
		if device.SerialNumber == serialNumberStr {
			return device
		}
	}
	return nil
}

func (sim *Simulator) replayFunc(d *Device, v map[string]interface{}, auth, version string) error {
	var errs = make(chan error, 2)
	go func() {
		errs <- getFirmware(d, v, auth, version)
	}()
	go func() {
		errs <- getFirmware(d, v, auth, version)
	}()

	for i := 0; i < 2; i++ {
		if err := <-errs; err != nil {
			log.Warnf("Replay %v: %v\n", d.SerialNumber, err)
			return nil
		}
	}
	return nil
}

// Simulate a replay attack for a specific device
func (sim *Simulator) Replay(serialNumber int) error {
	sim.log.Printf("Simulating replay attack for device %v\n", serialNumber)
	// Simulate replay logic here
	// For example, you can send a request to the device with the same parameters as before
	device := sim.GetDevice(serialNumber)
	if device == nil {
		return fmt.Errorf("device '%v' not found", serialNumber)
	}
	if err := device.Update(sim.replayFunc, "1.0.1"); err != nil {
		return err
	}
	return nil
}

// Simulate a batch replay attack for a range of devices
func (sim *Simulator) BatchReplay(startSerial, endSerial int) error {
	for i := startSerial; i <= endSerial; i++ {
		err := sim.Replay(i)
		if err != nil {
			sim.log.Printf("Failed to simulate replay attack for device %v: %v\n", i, err)
			return err
		}
	}
	return nil
}

func (sim *Simulator) Setup(ctx context.Context, args []string) error {
	// Define flags for the command line arguments
	f := flag.NewFlagSet(sim.name, flag.ContinueOnError)
	f.StringVar(&sim.cfg, "config", "./internal/app/simulator/apis.json", "Path to the configuration file")
	generateCount := f.Int("generate", 0, "Generate a specified number of devices")
	startSerial := f.Int("start-serial", -1, "Starting serial number for device generation")
	updateSerial := f.Int("update", -1, "Request update for a specific device")
	batchUpdateRange := f.String("batch-update", "", "Request updates for a range of devices (e.g., '100-200')")
	statusSerial := f.Int("status", -1, "Show status of a specific device")
	listAll := f.Bool("list-all", false, "List all simulated devices with their status")
	replaySerial := f.Int("simulate-replay", -1, "Simulate a replay attack for a specific device")
	batchReplayRange := f.String("simulate-batch-replay", "", "Simulate batch replay attacks for a range of devices")
	port := f.Int("port", 0, "Port for the simulator to run on")
	endpoint := f.String("endpoint", "127.0.0.1:9001", "Simulator address")
	server := f.String("server", "127.0.0.1:9000", "Server address")
	// Parse command line arguments
	err := f.Parse(args)
	if err != nil {
		return err
	}
	sim.ctx = ctx
	// Handle the different commands based on the flags
	var exe = NewExecuter(*endpoint)
	switch {
	case *generateCount > 0 && *startSerial >= 0:
		err := exe.GenerateDevices(*server, *generateCount, *startSerial)
		if err != nil {
			return err
		}
		fmt.Printf("Generate %v devices succeed\n", *generateCount)
		os.Exit(0)

	case *updateSerial > 0:
		err := exe.UpdateDevice(*updateSerial, "1.0.1")
		if err != nil {
			return err
		}
		fmt.Printf("Update device %v succeed\n", *updateSerial)
		os.Exit(0)

	case *batchUpdateRange != "":
		rangeParts := strings.Split(*batchUpdateRange, "-")
		if len(rangeParts) != 2 {
			return fmt.Errorf("invalid batch update range. Please use 'startSerial-endSerial'")
		}
		start, _ := strconv.Atoi(rangeParts[0])
		end, _ := strconv.Atoi(rangeParts[1])
		if start < 0 || end < 0 || end < start {
			return fmt.Errorf("invalid batch update range. Please use 'startSerial-endSerial'")
		}
		if err := exe.BatchUpdate(start, end, "1.0.1"); err != nil {
			return err
		}
		fmt.Printf("Update devices %v succeed\n", *batchUpdateRange)
		os.Exit(0)

	case *statusSerial >= 0:
		s, err := exe.GetDeviceStatus(*statusSerial)
		if err != nil {
			return err
		}
		data, _ := json.MarshalIndent(s, "", "  ")
		fmt.Printf("Device %v status: \n%s\n", *statusSerial, string(data))
		os.Exit(0)

	case *listAll:
		arr, err := exe.GetDeviceList()
		if err != nil {
			return err
		}
		data, _ := json.MarshalIndent(arr, "", "  ")
		fmt.Printf("Get device list: %d\n%s\n", len(arr), string(data))
		os.Exit(0)

	case *replaySerial >= 0:
		err := exe.Replay(*replaySerial)
		if err != nil {
			return err
		}
		fmt.Printf("Replay device %v succeed\n", *replaySerial)
		os.Exit(0)

	case *batchReplayRange != "":
		rangeParts := strings.Split(*batchReplayRange, "-")
		if len(rangeParts) != 2 {
			return fmt.Errorf("invalid batch replay range. Please use 'startSerial-endSerial'")
		}
		start, _ := strconv.Atoi(rangeParts[0])
		end, _ := strconv.Atoi(rangeParts[1])
		err := exe.BatchReplay(start, end)
		if err != nil {
			return err
		}
		fmt.Printf("Replay device %v succeed\n", *batchReplayRange)
		os.Exit(0)

	case *port > 0:
		sim.port = *port
		fmt.Printf("Simulator will run on port %d\n", sim.port)

	default:
		fmt.Println("Usage: simulator --generate=<count> --start-serial=<number>")
		fmt.Println("       simulator --update=<serialNumber>")
		fmt.Println("       simulator --batch-update=<startSerial>-<endSerial>")
		fmt.Println("       simulator --status=<serialNumber>")
		fmt.Println("       simulator --list-all")
		fmt.Println("       simulator --simulate-replay=<serialNumber>")
		fmt.Println("       simulator --simulate-batch-replay=<startSerial>-<endSerial>")
		os.Exit(1)
	}

	return nil
}

func (sim *Simulator) IsReady() bool {
	return sim.ready
}

func (sim *Simulator) SetReady(bool) {
	sim.ready = true
}

func (sim *Simulator) restoreDevices(ctx context.Context) ([]*Device, error) {
	var devices []*Device

	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".json") && len(file.Name()) == 15 {
			filePath := filepath.Join(dir, file.Name())
			data, err := os.ReadFile(filePath)
			if err != nil {
				return nil, err
			}
			var device Device
			if err := json.Unmarshal(data, &device); err != nil {
				return nil, err
			}
			device.SetSimulator(sim)
			devices = append(devices, &device)
			go device.RegisterProc(ctx, 5*time.Second)
		}
	}

	return devices, nil
}

func (sim *Simulator) Run(ctx context.Context) error {
	if sim.port <= 0 {
		return nil
	}
	// restore device list
	var err error
	sim.devices, err = sim.restoreDevices(ctx)
	if err != nil {
		sim.log.Printf("Failed to restore devices: %v\n", err)
	}
	logManager := audit.NewManager("sim_log.json")
	var log, _ = logging.NewLogger("INFO", os.Stdout, "")
	var srvConf = config.ServiceConfig{
		Version:         1,
		Name:            sim.name,
		Debug:           false,
		Timeout:         time.Duration(180) * time.Second,
		CacheTTL:        time.Duration(10) * time.Second,
		Port:            sim.port,
		SequentialStart: true,
		ExtraConfig:     map[string]interface{}{audit.LOG_MANAGER: logManager},
	}
	srvConf.Endpoints, err = readPluginFile(sim.cfg)
	if err != nil {
		return err
	}
	srvConf.NormalizeEndpoints()
	// Global plugin factory
	factory := map[string]vicg.VicgPluginFactory{
		"HttpData_Parse":   httpdata_parse.NewFactory(),
		"Device_Simulator": device_simulate.NewFactory(sim),
		"Request_Update":   request_update.NewFactory(sim),
		"Batch_Update":     batch_update.NewFactory(sim),
		"Replay_Simulate":  replay_simulate.NewFactory(sim),
		"Device_Status":    device_status.NewFactory(sim),
		"Device_List":      device_list.NewFactory(sim),
	}
	f := func(cfg *gin.Config) {
		pprof.Register(cfg.Engine) // register pprof
	}
	router := gin.DefaultVicgFactory(vicg.DefaultVicgFactory(log, factory), log, f).NewWithContext(ctx)
	router.Run(srvConf)

	return nil
}

func (sim *Simulator) Stop(context.Context) {

}

func (sim *Simulator) IsDebug() bool {
	return false
}

func readPluginFile(fileName string) ([]*config.EndpointConfig, error) {
	plugin := &config.EndpointPluginList{}
	file, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}

	defer file.Close()

	bytes, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(bytes, plugin)
	if err != nil {
		return nil, err
	}
	return plugin.Plugin, nil
}
