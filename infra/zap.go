package infra

import (
	"golang-ast/conf"
	"os"
	"time"

	"github.com/natefinch/lumberjack/v3"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func InitLogger(cfg *conf.LogConfig) *zap.Logger {
	writeSyncer := getLogWriter(cfg.Filename, cfg.MaxSize, cfg.MaxBackups, cfg.MaxAge)
	encoder := getEncoder()
	var l = new(zapcore.Level)
	_ = l.UnmarshalText([]byte(cfg.Level))
	core := zapcore.NewCore(encoder, writeSyncer, l)
	coreConsole := zapcore.NewCore(encoder, zapcore.AddSync(os.Stdout), l)
	logger := zap.New(zapcore.NewTee(core, coreConsole), zap.AddCaller())
	zap.ReplaceGlobals(logger) // 替换zap包中全局的logger实例，后续在其他包中只需使用zap.L()调用即可
	return logger
}

func getEncoder() zapcore.Encoder {
	config := zapcore.EncoderConfig{
		MessageKey:     "message",
		LevelKey:       "level",
		TimeKey:        "time",
		CallerKey:      "caller",
		NameKey:        "logger",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalColorLevelEncoder,
		EncodeTime:     zapcore.RFC3339TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
	return zapcore.NewConsoleEncoder(config)
}

func getLogWriter(filename string, maxSize, maxBackup, maxAge int) zapcore.WriteSyncer {
	lumberJackLogger, _ := lumberjack.NewRoller(filename, int64(maxSize*1024*1024), &lumberjack.Options{
		MaxBackups: maxBackup,
		MaxAge:     time.Duration(maxAge * int(time.Hour) * 24),
		Compress:   true,
	})
	return zapcore.AddSync(lumberJackLogger)
}
