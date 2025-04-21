package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/luraproject/lura/v2/core"
	luraServer "github.com/luraproject/lura/v2/transport/http/server"
	"github.com/yuanyuanxiang/fss/internal/app/server"
	"github.com/yuanyuanxiang/fss/internal/app/simulator"
	"github.com/yuanyuanxiang/fss/pkg/logger"
)

func main() {
	// Initialize logger
	lg, err := logger.NewLogger()
	if err != nil {
		log.Fatalf("Initialize logger error: %v\n", err)
	}

	// Receive system signals
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT)
	defer stop()

	// Set up LURA parameters
	core.KrakendVersion = "2.9.0"
	core.KrakendHeaderValue = fmt.Sprintf("Version %s", core.KrakendVersion)
	core.KrakendUserAgent = fmt.Sprintf("LURA Version %s", core.KrakendVersion)
	luraServer.HeadersToSend = []string{"*"}

	// Add submodules
	app := NewApp(filepath.Base(os.Args[0]), lg)
	app.AddModule(ctx, server.New("./configs/private_key.pem", lg))
	app.AddModule(ctx, simulator.New(lg, simulator.WithCertFile("./configs/cert.pem")))

	// Parse command line arguments and run the specified service
	commands := NewArgs(os.Args[1:])
	if !commands.Present() {
		lg.Errorf("Missing startup service name")
		return
	}
	m, err := app.SetupOne(ctx, commands.First(), commands.Tail())
	if err != nil {
		lg.Errorf("Get instance error: %v", logger.ErrorField(err))
		return
	}
	defer m.Stop(ctx)
	if err := m.Run(ctx); err != nil {
		lg.Errorf("Module running error: %v", logger.ErrorField(err))
		return
	}
}
