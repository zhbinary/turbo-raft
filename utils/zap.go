//Created by zhbinary on 2019-04-10.
//Email: zhbinary@gmail.com
package utils

import (
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"
)
import "go.uber.org/zap"

var (
	Logger = NewConsoleLogger("quick-raft")
	//Logger *zap.Logger
)

func NewZap() *zap.SugaredLogger {
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("can't initialize zap logger: %v", err)
	}
	defer logger.Sync()
	return logger.Sugar()
}

func NewConsoleLogger(tag string) *zap.Logger {
	w := zapcore.AddSync(os.Stdout)
	encoderConfig := zap.NewProductionEncoderConfig()
	//encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.Format("2006-01-02T15:04:05"))
	}
	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoderConfig),
		w,
		zapcore.DebugLevel,
	)

	var logger *zap.Logger
	//logger = zap.New(core).With(zap.String("", tag))
	logger = zap.New(core)
	return logger
}

func NewZapLogger(opt *LoggerConfig) (*zap.Logger, io.Writer) {
	if opt.MaxSize <= 0 {
		opt.MaxSize = 100
	}
	if opt.MaxBackup <= 0 {
		opt.MaxBackup = 10
	}
	if opt.MaxAge <= 0 {
		opt.MaxAge = 28
	}
	hook := lumberjack.Logger{
		Filename:   filepath.Join(opt.LogPath, opt.LogName),
		MaxSize:    opt.MaxSize,
		MaxBackups: opt.MaxBackup,
		MaxAge:     opt.MaxAge,
		LocalTime:  true,
		Compress:   true,
	}
	w := zapcore.AddSync(&hook)

	var level zapcore.Level
	switch opt.LogLevel {
	case DEBUG:
		level = zap.DebugLevel
	case INFO:
		level = zap.InfoLevel
	case WARN:
		level = zap.WarnLevel
	case ERROR:
		level = zap.ErrorLevel
	case FATAL:
		level = zap.FatalLevel
	default:
		level = zap.InfoLevel
	}

	encoderConfig := zap.NewProductionEncoderConfig()
	//encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.Format("2006-01-02T15:04:05"))
	}

	var core zapcore.Core
	if opt.LogOutput == FILE {
		core = zapcore.NewCore(
			zapcore.NewConsoleEncoder(encoderConfig),
			//zapcore.NewJSONEncoder(encoderConfig),
			w,
			level,
		)
	} else if opt.LogOutput == CONSOLE {
		core = zapcore.NewCore(
			zapcore.NewConsoleEncoder(encoderConfig),
			zapcore.AddSync(os.Stdout),
			level,
		)
	} else if opt.LogOutput == ALL {
		fileCore := zapcore.NewCore(
			zapcore.NewConsoleEncoder(encoderConfig),
			//zapcore.NewJSONEncoder(encoderConfig),
			w,
			level,
		)
		consoleCore := zapcore.NewCore(
			zapcore.NewConsoleEncoder(encoderConfig),
			zapcore.AddSync(os.Stdout),
			level,
		)
		core = zapcore.NewTee(fileCore, consoleCore)
	}

	var logger *zap.Logger
	if opt.LogLevel == DEBUG {
		logger = zap.New(core, zap.AddCaller())
	} else {
		logger = zap.New(core)
	}

	return logger, &hook
}

func Error(msg string, err error) {
	Logger.Error(msg, zap.String("err", err.Error()))
	debug.PrintStack()
}
