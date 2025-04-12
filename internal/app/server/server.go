package server

import (
	"context"
	"crypto/ecdh"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/gin-contrib/pprof"
	"github.com/luraproject/lura/v2/config"
	"github.com/luraproject/lura/v2/logging"
	"github.com/luraproject/lura/v2/router/gin"
	"github.com/luraproject/lura/v2/vicg"
	"github.com/yuanyuanxiang/fss/internal/pkg/common"
	"github.com/yuanyuanxiang/fss/pkg/audit"
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
	endpoint := f.String("endpoint", "127.0.0.1:9000", "Server address")

	err := f.Parse(args)
	if err != nil {
		return err
	}
	var exe = NewExecuter(*endpoint)
	switch {
	case *port > 0 && *allowance > 0:
		svr.port = *port
		svr.allowance = *allowance
		fmt.Printf("Server started on port %d, allowance: %d\n", *port, *allowance)

	case *increaseAllowance > 0:
		allow, err := exe.IncreaseAllowance("", *increaseAllowance)
		if err != nil {
			return err
		}
		fmt.Printf("Increasing allowance by %d succeed. Current allowance: %d\n", *increaseAllowance, allow)
		os.Exit(0)

	case *listDevices:
		list, err := exe.GetDeviceList()
		if err != nil {
			return err
		}
		data, _ := json.MarshalIndent(list, "", "  ")
		fmt.Printf("Registered devices: %d\n%s\n", len(list), string(data))
		os.Exit(0)

	case *showIncidents:
		list, err := exe.GetAuditLogs(string(audit.TYPE_INCIDENT))
		if err != nil {
			return err
		}
		data, _ := json.MarshalIndent(list, "", "  ")
		fmt.Printf("Security incidents: %d\n%s\n", len(list), string(data))
		os.Exit(0)

	case *showUpdates:
		list, err := exe.GetAuditLogs(string(audit.TYPE_UPDATE))
		if err != nil {
			return err
		}
		data, _ := json.MarshalIndent(list, "", "  ")
		fmt.Printf("Update logs: %d\n%s\n", len(list), string(data))
		os.Exit(0)

	case *block != "":
		if err := exe.BlockDevice(*block); err != nil {
			return err
		}
		fmt.Println("Succeed blocking device: ", *block)
		os.Exit(0)

	case *authorize != "":
		if err := exe.AuthorizeDevice(*block); err != nil {
			return err
		}
		fmt.Println("Succeed authorizing device: ", *authorize)
		os.Exit(0)

	default:
		fmt.Println("Usage: server --port=<port> - Start the server on specified port")
		fmt.Println("       server --allowance=<number> - Set initial device registration allowance")
		fmt.Println("       server --increase-allowance=<number> - Increase allowance counter by specified amount")
		fmt.Println("       server --list-devices - Display all registered devices")
		fmt.Println("       server --show-incidents - Display security incident logs")
		fmt.Println("       server --show-updates - Display successful update logs")
		fmt.Println("       server --block=<serialNumber> - Block a specific device")
		fmt.Println("       server --authorize=<serialNumber> - Authorize a specific device")
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
	logManager := audit.NewManager("svr_log.json")
	var log, _ = logging.NewLogger("INFO", os.Stdout, "")
	var srvConf = config.ServiceConfig{
		Version:         1,
		Name:            svr.name,
		Debug:           false,
		Timeout:         time.Duration(180) * time.Second,
		CacheTTL:        time.Duration(10) * time.Second,
		Port:            svr.port,
		SequentialStart: true,
		ExtraConfig:     map[string]interface{}{audit.LOG_MANAGER: logManager}, // pass log manager to all plugins
	}
	var err error
	srvConf.Endpoints, err = readPluginFile(svr.cfg)
	if err != nil {
		return err
	}
	srvConf.NormalizeEndpoints()
	sessManeger := NewSessionManager()
	devManager := NewDeviceManager(svr.allowance)
	// Global plugin factory
	factory := map[string]vicg.VicgPluginFactory{
		"HttpData_Parse":   httpdata_parse.NewFactory(),
		"Challenge_Gen":    challenge_gen.NewFactory(sessManeger),
		"Challenge_Verify": challenge_verify.NewFactory(sessManeger, devManager, common.SymmetricKey),
		"Device_Register":  device_register.NewFactory(sessManeger, devManager, common.PublicKeyToBase64(svr.key.PublicKey())),
		"Allowance_Update": allowance_update.NewFactory(devManager),
		"Firmware_Update":  firmware_update.NewFactory(sessManeger, devManager, svr.key),
		"Device_List":      device_list.NewFactory(devManager),
		"Device_Auth":      device_auth.NewFactory(devManager),
		"Audit_Logs":       audit_logs.NewFactory(),
	}
	f := func(cfg *gin.Config) {
		pprof.Register(cfg.Engine) // register pprof
	}
	router := gin.DefaultVicgFactory(vicg.DefaultVicgFactory(log, factory), log, f).NewWithContext(ctx)
	router.Run(srvConf)

	return nil
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
