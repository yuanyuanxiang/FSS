package simulator

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/pprof"
	"github.com/luraproject/lura/v2/config"
	"github.com/luraproject/lura/v2/logging"
	"github.com/luraproject/lura/v2/router/gin"
	"github.com/luraproject/lura/v2/vicg"
	"github.com/yuanyuanxiang/fss/internal/pkg/common"
	"github.com/yuanyuanxiang/fss/pkg/logger"
	"github.com/yuanyuanxiang/fss/plugins/batch_update"
	"github.com/yuanyuanxiang/fss/plugins/device_list"
	"github.com/yuanyuanxiang/fss/plugins/device_simulate"
	"github.com/yuanyuanxiang/fss/plugins/device_status"
	"github.com/yuanyuanxiang/fss/plugins/httpdata_parse"
	"github.com/yuanyuanxiang/fss/plugins/replay_simulate"
	"github.com/yuanyuanxiang/fss/plugins/request_update"
)

// Simulator application
type Simulator struct {
	name    string // Module name
	cfg     string // Configuration file path
	log     logger.Logger
	devices []*Device
	port    int
	ready   bool
}

func New(log logger.Logger) *Simulator {
	return &Simulator{log: log, name: "simulator"}
}

func (sim *Simulator) GetName() string {
	return sim.name
}

// Generate specified number of devices
func (sim *Simulator) GenerateDevices(master string, count int, startSerial int) error {
	for i := 0; i < count; i++ {
		id := i + startSerial
		device, err := NewDevice(master, id, "1.0.0", common.SymmetricKey)
		if err != nil {
			sim.log.Printf("Failed to generate device %v: %v\n", id, err)
			continue
		}
		sim.devices = append(sim.devices, device)
		go func() {
			if err := device.Register(); err != nil {
				sim.log.Printf("Failed to register device %s: %v\n", device.SerialNumber, err)
			}
		}()
		sim.log.Printf("Generated device: SerialNumber=%v\n", id)
	}
	return nil
}

// Request update for a specific device
func (sim *Simulator) UpdateDevice(serialNumber int) error {
	serialNumberStr := fmt.Sprintf("%010d", serialNumber)
	for _, device := range sim.devices {
		if device.SerialNumber == serialNumberStr {
			return device.Update("1.0.1")
		}
	}
	sim.log.Printf("Device with serial number %v not found.\n", serialNumber)
	return fmt.Errorf("device not found")
}

// Request updates for a range of devices
func (sim *Simulator) BatchUpdate(startSerial, endSerial int) error {
	for i := startSerial; i <= endSerial; i++ {
		_ = sim.UpdateDevice(i)
	}
	return nil
}

// Show the status of a specific device
func (sim *Simulator) GetDeviceStatus(serialNumber int) (map[string]interface{}, error) {
	serialNumberStr := fmt.Sprintf("%010d", serialNumber)
	for _, device := range sim.devices {
		if device.SerialNumber == serialNumberStr {
			sim.log.Printf("Device %v: FirmwareVersion=%s, State=%s\n", serialNumber, device.FirmwareVersion, device.State)
			return map[string]interface{}{"serial_number": serialNumberStr, "firmware_version": device.FirmwareVersion, "state": device.State}, nil
		}
	}
	sim.log.Printf("Device with serial number %v not found.\n", serialNumber)
	return nil, fmt.Errorf("device not found")
}

// List all devices with their status
func (sim *Simulator) GetDeviceList() ([]map[string]interface{}, error) {
	var deviceList []map[string]interface{}
	for _, device := range sim.devices {
		sim.log.Printf("Device %s: FirmwareVersion=%s, State=%s\n", device.SerialNumber, device.FirmwareVersion, device.State)
		deviceList = append(deviceList, map[string]interface{}{
			"serial_number":    device.SerialNumber,
			"firmware_version": device.FirmwareVersion,
			"state":            device.State,
		})
	}
	return deviceList, nil
}

// Simulate a replay attack for a specific device
func (sim *Simulator) Replay(serialNumber int) error {
	sim.log.Printf("Simulating replay attack for device %v\n", serialNumber)
	// Simulate replay logic here
	return nil
}

// Simulate a batch replay attack for a range of devices
func (sim *Simulator) simulateBatchReplay(startSerial, endSerial int) {
	for i := startSerial; i <= endSerial; i++ {
		err := sim.Replay(i)
		if err != nil {
			sim.log.Printf("Failed to simulate replay attack for device %v: %v\n", i, err)
			continue
		}
	}
}

func (sim *Simulator) Setup(ctx context.Context, args []string) error {
	// Define flags for the command line arguments
	f := flag.NewFlagSet(sim.name, flag.ContinueOnError)
	f.StringVar(&sim.cfg, "config", "D:\\github\\FSS\\internal\\app\\simulator\\apis.json", "Path to the configuration file")
	generateCount := f.Int("generate", 0, "Generate a specified number of devices")
	startSerial := f.Int("start-serial", 0, "Starting serial number for device generation")
	updateSerial := f.Int("update", 0, "Request update for a specific device")
	batchUpdateRange := f.String("batch-update", "", "Request updates for a range of devices (e.g., '100-200')")
	statusSerial := f.Int("status", 0, "Show status of a specific device")
	listAll := f.Bool("list-all", false, "List all simulated devices with their status")
	replaySerial := f.Int("simulate-replay", 0, "Simulate a replay attack for a specific device")
	batchReplayRange := f.String("simulate-batch-replay", "", "Simulate batch replay attacks for a range of devices")
	port := f.Int("port", 0, "Port for the simulator to run on")
	// Parse command line arguments
	err := f.Parse(args)
	if err != nil {
		return err
	}

	// Handle the different commands based on the flags
	if *generateCount > 0 {
		_ = sim.GenerateDevices("127.0.0.1:9000", *generateCount, *startSerial)
	} else if *updateSerial > 0 {
		_ = sim.UpdateDevice(*updateSerial)
	} else if *batchUpdateRange != "" {
		rangeParts := strings.Split(*batchUpdateRange, "-")
		if len(rangeParts) == 2 {
			start, _ := strconv.Atoi(rangeParts[0])
			end, _ := strconv.Atoi(rangeParts[1])
			_ = sim.BatchUpdate(start, end)
		} else {
			fmt.Println("Invalid batch update range. Please use 'startSerial-endSerial'.")
		}
	} else if *statusSerial > 0 {
		s, err := sim.GetDeviceStatus(*statusSerial)
		if err == nil {
			fmt.Println(s)
		}
	} else if *listAll {
		a, err := sim.GetDeviceList()
		if err == nil {
			fmt.Println(a)
		}
	} else if *replaySerial > 0 {
		_ = sim.Replay(*replaySerial)
	} else if *batchReplayRange != "" {
		rangeParts := strings.Split(*batchReplayRange, "-")
		if len(rangeParts) == 2 {
			start, _ := strconv.Atoi(rangeParts[0])
			end, _ := strconv.Atoi(rangeParts[1])
			sim.simulateBatchReplay(start, end)
		} else {
			fmt.Println("Invalid batch replay range. Please use 'startSerial-endSerial'.")
		}
	} else if *port > 0 {
		sim.port = *port
		fmt.Printf("Simulator will run on port %d\n", sim.port)
	} else {
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

func (sim *Simulator) restoreDevices() ([]*Device, error) {
	var devices []*Device

	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".json") && len(file.Name()) == 15 {
			filePath := filepath.Join(dir, file.Name())
			data, err := ioutil.ReadFile(filePath)
			if err != nil {
				return nil, err
			}
			var device Device
			if err := json.Unmarshal(data, &device); err != nil {
				return nil, err
			}

			devices = append(devices, &device)
			go func() {
				if err := device.Register(); err != nil {
					sim.log.Printf("Failed to register device %s: %v\n", device.SerialNumber, err)
				}
			}()
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
	sim.devices, err = sim.restoreDevices()
	if err != nil {
		sim.log.Printf("Failed to restore devices: %v\n", err)
	}
	var log, _ = logging.NewLogger("INFO", os.Stdout, "")
	var srvConf = config.ServiceConfig{
		Version:         1,
		Name:            sim.name,
		Debug:           false,
		Timeout:         time.Duration(180) * time.Second,
		CacheTTL:        time.Duration(10) * time.Second,
		Port:            sim.port,
		SequentialStart: true,
		ExtraConfig:     map[string]interface{}{"Hello": "world"},
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
