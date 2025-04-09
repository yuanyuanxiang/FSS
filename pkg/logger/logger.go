package logger

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	timeFormat  = "060102 15:04:05.0000"
	LoggerURL   = "/api/logger/level"
	defaultSkip = 1 // Default number of stack frames to skip
)

var (
	outputPaths  = []string{"stderr"}
	globalOnce   sync.Once
	globalLogger Logger
)

// Field defines the log field type
type Field struct {
	Key   string
	Value interface{}
}

func MakeField(k string, v interface{}) Field {
	return Field{Key: k, Value: v}
}

// ErrorField returns a log field for error
func ErrorField(err error) Field {
	return Field{Key: "error", Value: err}
}

// Level defines the log level type
type Level zapcore.Level

const (
	DebugLevel Level = iota - 1
	InfoLevel
	WarnLevel
	ErrorLevel
	DPanicLevel
	PanicLevel
	FatalLevel
)

// Logger is the logging interface
type Logger interface {
	RegisterAPI(*gin.Engine)
	Handler() http.Handler
	SetLevel(Level)
	GetLevel() string

	Critical(v ...interface{})

	// Structured logging methods
	Debug(msg string, fields ...Field)
	Info(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
	Error(msg string, fields ...Field)
	DPanic(msg string, fields ...Field)
	Panic(msg string, fields ...Field)
	Fatal(msg string, fields ...Field)

	// Formatted logging methods
	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	DPanicf(format string, args ...interface{})
	Panicf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})

	// Standard library compatibility methods
	Print(v ...interface{})
	Printf(format string, v ...interface{})
	Println(v ...interface{})

	// Log level checks
	IsDebug() bool
	With(fields ...Field) Logger
}

// AppLogger is the implementation of the Logger interface
type AppLogger struct {
	cfg    *zap.Config
	slg    *zap.SugaredLogger
	lg     *zap.Logger
	level  zap.AtomicLevel
	fields []zap.Field
}

// RegisterAPI registers the API for log level control
func (l *AppLogger) RegisterAPI(eng *gin.Engine) {
	eng.GET(LoggerURL, gin.WrapH(l.level))
	eng.PUT(LoggerURL, gin.WrapH(l.level))
}

func (l *AppLogger) Handler() http.Handler {
	return l.level
}

func (l *AppLogger) SetLevel(lv Level) {
	l.level.SetLevel(zapcore.Level(lv))
}

func (l *AppLogger) GetLevel() string {
	return l.level.Level().CapitalString()
}

func (l *AppLogger) Critical(v ...interface{}) {
	l.slg.Fatal(v...)
}

// Structured logging methods
func (l *AppLogger) Debug(msg string, fields ...Field) {
	l.log(zapcore.DebugLevel, msg, fields)
}

func (l *AppLogger) Info(msg string, fields ...Field) {
	l.log(zapcore.InfoLevel, msg, fields)
}

func (l *AppLogger) Warn(msg string, fields ...Field) {
	l.log(zapcore.WarnLevel, msg, fields)
}

func (l *AppLogger) Error(msg string, fields ...Field) {
	l.log(zapcore.ErrorLevel, msg, fields)
}

func (l *AppLogger) DPanic(msg string, fields ...Field) {
	l.log(zapcore.DPanicLevel, msg, fields)
}

func (l *AppLogger) Panic(msg string, fields ...Field) {
	l.log(zapcore.PanicLevel, msg, fields)
}

func (l *AppLogger) Fatal(msg string, fields ...Field) {
	l.log(zapcore.FatalLevel, msg, fields)
}

// Formatted logging methods
func (l *AppLogger) Debugf(format string, args ...interface{}) {
	l.slg.Debugf(format, args...)
}

func (l *AppLogger) Infof(format string, args ...interface{}) {
	l.slg.Infof(format, args...)
}

func (l *AppLogger) Warnf(format string, args ...interface{}) {
	l.slg.Warnf(format, args...)
}

func (l *AppLogger) Errorf(format string, args ...interface{}) {
	l.slg.Errorf(format, args...)
}

func (l *AppLogger) DPanicf(format string, args ...interface{}) {
	l.slg.DPanicf(format, args...)
}

func (l *AppLogger) Panicf(format string, args ...interface{}) {
	l.slg.Panicf(format, args...)
}

func (l *AppLogger) Fatalf(format string, args ...interface{}) {
	l.slg.Fatalf(format, args...)
}

// Standard library compatibility methods
func (l *AppLogger) Print(v ...interface{}) {
	l.slg.Info(v...)
}

func (l *AppLogger) Printf(format string, v ...interface{}) {
	l.slg.Infof(format, v...)
}

func (l *AppLogger) Println(v ...interface{}) {
	l.slg.Info(v...)
}

// IsDebug checks if the current level is debug or lower
func (l *AppLogger) IsDebug() bool {
	return l.level.Level() <= zapcore.DebugLevel
}

// With creates a new logger with predefined fields
func (l *AppLogger) With(fields ...Field) Logger {
	zapFields := make([]zap.Field, len(l.fields), len(l.fields)+len(fields))
	copy(zapFields, l.fields)

	for _, f := range fields {
		zapFields = append(zapFields, zap.Any(f.Key, f.Value))
	}

	newLogger := l.lg.With(zapFields...)
	return &AppLogger{
		cfg:    l.cfg,
		slg:    newLogger.Sugar(),
		lg:     newLogger,
		level:  l.level,
		fields: zapFields,
	}
}

// log is the internal logging function
func (l *AppLogger) log(lvl zapcore.Level, msg string, fields []Field) {
	if !l.level.Enabled(lvl) {
		return
	}

	zapFields := make([]zap.Field, 0, len(fields)+len(l.fields))
	zapFields = append(zapFields, l.fields...)

	for _, f := range fields {
		zapFields = append(zapFields, zap.Any(f.Key, f.Value))
	}

	if ce := l.lg.Check(lvl, msg); ce != nil {
		ce.Write(zapFields...)
	}
}

// Option is a functional option for configuring logger
type Option func(*zap.Config)

// SetDebug sets development mode
func SetDebug(debug bool) Option {
	return func(conf *zap.Config) {
		conf.Development = debug
		if debug {
			conf.Encoding = "console"
		} else {
			conf.Encoding = "json"
		}
	}
}

// SetLevel sets the log level
func SetLevel(lv Level) Option {
	return func(conf *zap.Config) {
		conf.Level = zap.NewAtomicLevelAt(zapcore.Level(lv))
	}
}

// SetOutput sets the output paths
func SetOutput(paths []string) Option {
	return func(conf *zap.Config) {
		conf.OutputPaths = paths
		conf.ErrorOutputPaths = paths
	}
}

// NewLogger creates a new Logger instance
func NewLogger(opts ...Option) (Logger, error) {
	var initErr error
	globalOnce.Do(func() {
		cfg := defaultConfig()
		for _, opt := range opts {
			opt(&cfg)
		}

		logger, err := cfg.Build(zap.AddCallerSkip(defaultSkip))
		if err != nil {
			initErr = err
			return
		}

		globalLogger = &AppLogger{
			cfg:   &cfg,
			slg:   logger.Sugar(),
			lg:    logger,
			level: cfg.Level,
		}
	})

	return globalLogger, initErr
}

// defaultConfig returns the default logger configuration
func defaultConfig() zap.Config {
	return zap.Config{
		Level:       zap.NewAtomicLevelAt(zapcore.InfoLevel),
		Development: false,
		Sampling: &zap.SamplingConfig{
			Initial:    100,
			Thereafter: 100,
		},
		Encoding: "json",
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "ts",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			FunctionKey:    zapcore.OmitKey,
			MessageKey:     "msg",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.TimeEncoderOfLayout(timeFormat),
			EncodeDuration: zapcore.StringDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
		OutputPaths:      outputPaths,
		ErrorOutputPaths: outputPaths,
	}
}
