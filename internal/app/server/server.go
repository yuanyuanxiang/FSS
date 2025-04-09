package server

import (
	"context"
	"crypto/ecdh"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-contrib/pprof"
	"github.com/luraproject/lura/v2/config"
	"github.com/luraproject/lura/v2/logging"
	"github.com/luraproject/lura/v2/router/gin"
	"github.com/luraproject/lura/v2/vicg"
	"github.com/yuanyuanxiang/fss/internal/pkg/common"
	"github.com/yuanyuanxiang/fss/pkg/logger"
	"github.com/yuanyuanxiang/fss/plugins/allowance_update"
	"github.com/yuanyuanxiang/fss/plugins/audit_logs"
	"github.com/yuanyuanxiang/fss/plugins/challenge_gen"
	"github.com/yuanyuanxiang/fss/plugins/challenge_verify"
	"github.com/yuanyuanxiang/fss/plugins/device_auth"
	"github.com/yuanyuanxiang/fss/plugins/device_list"
	"github.com/yuanyuanxiang/fss/plugins/device_register"
	"github.com/yuanyuanxiang/fss/plugins/firmware_update"
	"github.com/yuanyuanxiang/fss/plugins/httpdata_parse"
)

// Server application
type Server struct {
	mu        sync.Mutex
	name      string // Module name
	cfg       string
	logger    logger.Logger
	keyPath   string
	key       *ecdh.PrivateKey
	port      int
	allowance int
	ready     bool
}

func New(privateKeyPath string, logger logger.Logger) *Server {
	return &Server{name: "server", logger: logger, keyPath: privateKeyPath}
}

func (svr *Server) GetName() string {
	return svr.name
}

func (svr *Server) Setup(ctx context.Context, args []string) error {
	// Define flags for the command line arguments
	f := flag.NewFlagSet(svr.name, flag.ContinueOnError)
	f.StringVar(&svr.cfg, "config", "D:\\github\\FSS\\internal\\app\\server\\apis.json", "Path to the configuration file")
	port := f.Int("port", 0, "Start the server on specified port")
	allowance := f.Int("allowance", 0, "Set initial device registration allowance")
	increaseAllowance := f.Int("increase-allowance", 0, "Increase allowance counter")
	block := f.String("block", "", "Block a specific device")
	authorize := f.String("authorize", "", "Authorize a specific device")
	listDevices := f.Bool("list-devices", false, "List all registered devices")
	showIncidents := f.Bool("show-incidents", false, "Show security incident logs")
	showUpdates := f.Bool("show-updates", false, "Show successful update logs")

	err := f.Parse(args)
	if err != nil {
		return err
	}

	if *port > 0 && *allowance > 0 {
		svr.port = *port
		svr.allowance = *allowance
		fmt.Printf("Server started on port %d, allowance: %d", *port, *allowance)
	} else if *increaseAllowance > 0 {
		fmt.Println("Increasing allowance by:", *increaseAllowance)
	} else if *block != "" {
		fmt.Println("Blocking device:", *block)
	} else if *authorize != "" {
		fmt.Println("Authorizing device:", *authorize)
	} else if *listDevices {
		fmt.Println("Registered devices:")
	} else if *showIncidents {
		fmt.Println("Security incidents:")
	} else if *showUpdates {
		fmt.Println("Update logs:")
	}

	if len(flag.Args()) > 0 {
		fmt.Println("Unknown arguments:", strings.Join(flag.Args(), " "))
		os.Exit(1)
	}

	privKey, err := getOrCreatePrivateKey(svr.keyPath)
	if err != nil {
		return fmt.Errorf("failed to load or generate private key: %w", err)
	}
	svr.key = privKey
	svr.logger.Println("✅ Private key loaded or generated successfully:", svr.keyPath)

	flag.Parse()
	if svr.port <= 0 || svr.allowance <= 0 {
		return fmt.Errorf("invalid port number: %d or allowance number: %d", svr.port, svr.allowance)
	}

	svr.logger.Println("✅ Server setup completed. Port:", svr.port, "Allowance:", svr.allowance)
	return nil
}

func (svr *Server) IsReady() bool {
	return svr.ready
}

func (svr *Server) SetReady(bool) {
	svr.ready = true
}

func (svr *Server) Run(ctx context.Context) error {
	if svr.port <= 0 {
		return nil
	}

	var log, _ = logging.NewLogger("INFO", os.Stdout, "")
	var srvConf = config.ServiceConfig{
		Version:         1,
		Name:            svr.name,
		Debug:           false,
		Timeout:         time.Duration(180) * time.Second,
		CacheTTL:        time.Duration(10) * time.Second,
		Port:            svr.port,
		SequentialStart: true,
		ExtraConfig:     map[string]interface{}{"Hello": "world"},
	}
	var err error
	srvConf.Endpoints, err = readPluginFile(svr.cfg)
	if err != nil {
		return err
	}
	srvConf.NormalizeEndpoints()
	sessManeger := NewSessionManager()
	devManager := NewDeviceManager()
	// Global plugin factory
	factory := map[string]vicg.VicgPluginFactory{
		"HttpData_Parse":   httpdata_parse.NewFactory(),
		"Challenge_Gen":    challenge_gen.NewFactory(sessManeger),
		"Challenge_Verify": challenge_verify.NewFactory(sessManeger, svr, common.SymmetricKey),
		"Device_Register":  device_register.NewFactory(sessManeger, devManager, common.PublicKeyToBase64(svr.key.PublicKey())),
		"Allowance_Update": allowance_update.NewFactory(svr),
		"Firmware_Update":  firmware_update.NewFactory(devManager, svr.key),
		"Device_List":      device_list.NewFactory(devManager),
		"Device_Auth":      device_auth.NewFactory(devManager),
		"Audit_Logs":       audit_logs.NewFactory(svr),
	}
	f := func(cfg *gin.Config) {
		pprof.Register(cfg.Engine) // register pprof
	}
	router := gin.DefaultVicgFactory(vicg.DefaultVicgFactory(log, factory), log, f).NewWithContext(ctx)
	router.Run(srvConf)

	return nil
}

func (svr *Server) GetAuditLogs(typ string) ([]map[string]interface{}, error) {
	return nil, nil
}

func (svr *Server) GetAllowance(key string) int {
	svr.mu.Lock()
	defer svr.mu.Unlock()
	return svr.allowance
}

func (svr *Server) IncreaseAllowance(key string, inc int) {
	svr.mu.Lock()
	defer svr.mu.Unlock()
	svr.allowance += inc
}

func (svr *Server) Stop(context.Context) {

}

func (svr *Server) IsDebug() bool {
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
