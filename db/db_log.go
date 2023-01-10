package db

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
	gormlog "gorm.io/gorm/logger"
)

type Logger struct {
	ZapLogger                 *zap.Logger
	LogLevel                  gormlog.LogLevel
	SlowThreshold             time.Duration
	SkipCallerLookup          bool
	IgnoreRecordNotFoundError bool
}

func NewGormLog(zapLogger *zap.Logger) Logger {
	return Logger{
		ZapLogger:                 zapLogger,
		LogLevel:                  gormlog.Warn,
		SlowThreshold:             100 * time.Millisecond,
		SkipCallerLookup:          false,
		IgnoreRecordNotFoundError: true,
	}
}

func (l Logger) SetAsDefault() {
	gormlog.Default = l
}

func (l Logger) LogMode(level gormlog.LogLevel) gormlog.Interface {
	return Logger{
		ZapLogger:                 l.ZapLogger,
		SlowThreshold:             l.SlowThreshold,
		LogLevel:                  l.LogLevel,
		SkipCallerLookup:          l.SkipCallerLookup,
		IgnoreRecordNotFoundError: l.IgnoreRecordNotFoundError,
	}
}

func (l Logger) Info(ctx context.Context, str string, args ...interface{}) {
	if l.LogLevel < gormlog.Info {
		return
	}
	l.logger().Sugar().Named("\u001B[33m[GORM]\u001B[0m").Debugf(str, args...)
}

func (l Logger) Warn(ctx context.Context, str string, args ...interface{}) {
	if l.LogLevel < gormlog.Warn {
		return
	}
	l.logger().Sugar().Named("\u001B[33m[GORM]\u001B[0m").Warnf(str, args...)
}

func (l Logger) Error(ctx context.Context, str string, args ...interface{}) {
	if l.LogLevel < gormlog.Error {
		return
	}
	l.logger().Sugar().Named("\u001B[33m[GORM]\u001B[0m").Errorf(str, args...)
}

func (l Logger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.LogLevel <= 0 {
		return
	}
	elapsed := time.Since(begin)
	switch {
	case err != nil && l.LogLevel >= gormlog.Error && (!l.IgnoreRecordNotFoundError || !errors.Is(err, gorm.ErrRecordNotFound)):
		sql, rows := fc()
		l.logger().Named("\u001B[33m[GORM]\u001B[0m").Error("error", zap.Error(err), zap.Duration("elapsed", elapsed), zap.Int64("rows", rows), zap.String("sql", sql))
	case l.SlowThreshold != 0 && elapsed > l.SlowThreshold && l.LogLevel >= gormlog.Warn:
		sql, rows := fc()
		l.logger().Named("\u001B[33m[GORM]\u001B[0m").Warn("warn", zap.Duration("elapsed", elapsed), zap.Int64("rows", rows), zap.String("sql", sql))
	case l.LogLevel >= gormlog.Info:
		sql, rows := fc()
		l.logger().Named("\u001B[33m[GORM]\u001B[0m").Debug("debug", zap.Duration("elapsed", elapsed), zap.Int64("rows", rows), zap.String("sql", sql))
	}
}

func (l Logger) logger() *zap.Logger {
	return l.ZapLogger
}
