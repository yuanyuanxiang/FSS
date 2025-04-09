package main

import (
	"context"
	"fmt"

	"github.com/yuanyuanxiang/fss/pkg/logger"
)

// Module: module interface
type Module interface {
	GetName() string
	Setup(context.Context, []string) error
	IsReady() bool
	SetReady(bool)
	Run(context.Context) error
	Stop(context.Context)
	IsDebug() bool
}

// App: define the main process
type App struct {
	Logger  logger.Logger
	modules map[string]Module
	Name    string
}

// NewApp create a new App instance
func NewApp(appName string, lg logger.Logger) *App {
	app := &App{
		Name:    appName,
		modules: map[string]Module{},
		Logger:  lg,
	}
	return app
}

// Setup setup all registered modules
func (a *App) Setup(ctx context.Context, args []string) error {
	for _, item := range a.modules {
		if !item.IsReady() {
			if err := item.Setup(ctx, args); err != nil {
				a.Logger.Errorf("module '%s' setup error: %v", item.GetName(), err)
			}
		}
		item.SetReady(true)
		a.Logger.Infof("module '%s' ready", item.GetName())
	}
	return nil
}

// SetupOne setup a specified module
func (a *App) SetupOne(ctx context.Context, name string, args []string) (Module, error) {
	m, ok := a.modules[name]
	if !ok {
		return nil, fmt.Errorf("no registered module '%s'", name)
	}
	if ok && !m.IsReady() {
		if err := m.Setup(ctx, args); err != nil {
			return nil, err
		}
		if m.IsDebug() {
			a.Logger.SetLevel(logger.DebugLevel)
		}
		m.SetReady(true)
		a.Logger.Infof("module '%s' ready", m.GetName())
	}
	return m, nil
}

// AddModule add specified module to main process
func (a *App) AddModule(ctx context.Context, m Module) {
	a.modules[m.GetName()] = m
	a.Logger.Infof("module '%s' registered", m.GetName())
}

// Run run all registered modules
func (a *App) Run(ctx context.Context) {
	for _, item := range a.modules {
		if item.IsReady() {
			if err := item.Run(ctx); err != nil {
				a.Logger.Errorf("run module '%s' error: %v", item.GetName(), err)
				break
			}
			a.Logger.Infof("module '%s' running", item.GetName())
		}
	}
}

// Stop stop all registered modules
func (a *App) Stop(ctx context.Context) {
	for _, item := range a.modules {
		item.Stop(ctx)
		a.Logger.Infof("module '%s' stoped", item.GetName())
	}
}

// StopOne stop a specified module
func (a *App) StopOne(ctx context.Context, name string) {
	m, ok := a.modules[name]
	if ok {
		m.Stop(ctx)
		a.Logger.Infof("module '%s' stoped", m.GetName())
	}
}

// GetModules get all registered module names
func (a *App) GetModules() (modules []string) {
	for _, item := range a.modules {
		modules = append(modules, item.GetName())
	}
	return
}
