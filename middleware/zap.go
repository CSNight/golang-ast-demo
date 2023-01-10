package middleware

import (
	"golang-ast/conf"
	"golang-ast/infra"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

// LogConfig defines the config for middleware.
type LogConfig struct {
	// Next defines a function to skip this middleware when returned true.
	// Optional. Default: nil
	Next func(c *fiber.Ctx) bool
	// Add custom zap logger.
	// Optional. Default: zap.NewProduction()\n
	Logger *zap.Logger
	// Add fields what you want see.
	// Optional. Default: {"latency", "status", "method", "url"}
	Fields []string
	// Custom response messages.
	// Optional. Default: {"Server error", "Client error", "Success"}
	Messages []string

	OpLogCfg *conf.AuthConfig
}

// NewFiberLog New creates a new middleware handler
func NewFiberLog(cfg LogConfig) fiber.Handler {
	// Set PID once
	pid := strconv.Itoa(os.Getpid())
	trie := infra.NewTrie()
	for _, v := range cfg.OpLogCfg.Permits.Authentications {
		trie.Parse("/api"+v.Url, v.Permit)
	}
	for _, k := range cfg.OpLogCfg.Permits.WhiteList {
		trie.Parse("/api"+k, k)
	}
	// Set variables
	var (
		once       sync.Once
		errHandler fiber.ErrorHandler
	)

	errPadding := 15
	latencyEnabled := contains("latency", cfg.Fields)

	// Return new handler
	return func(c *fiber.Ctx) (err error) {
		// Don't execute middleware if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		// Set error handler once
		once.Do(func() {
			// get the longest possible path
			stack := c.App().Stack()
			for m := range stack {
				for r := range stack[m] {
					if len(stack[m][r].Path) > errPadding {
						errPadding = len(stack[m][r].Path)
					}
				}
			}
			// override error handler
			errHandler = c.App().Config().ErrorHandler
		})

		var start, stop time.Time

		if latencyEnabled {
			start = time.Now()
		}
		// Add fields
		fields := make([]zap.Field, 0, len(cfg.Fields))
		ips := c.Get(fiber.HeaderXForwardedFor)
		urlPath := c.Path()
		for _, field := range cfg.Fields {
			switch field {
			case "referer":
				fields = append(fields, zap.String("referer", c.Get(fiber.HeaderReferer)))
			case "protocol":
				fields = append(fields, zap.String("protocol", c.Protocol()))
			case "pid":
				fields = append(fields, zap.String("pid", pid))
			case "port":
				fields = append(fields, zap.String("port", c.Port()))
			case "ip":
				fields = append(fields, zap.String("ip", c.IP()))
			case "ips":
				fields = append(fields, zap.String("ips", ips))
			case "host":
				fields = append(fields, zap.String("host", c.Hostname()))
			case "path":
				fields = append(fields, zap.String("path", urlPath))
			case "url":
				fields = append(fields, zap.String("url", c.OriginalURL()))
			case "ua":
				fields = append(fields, zap.String("ua", c.Get(fiber.HeaderUserAgent)))
			case "queryParams":
				fields = append(fields, zap.String("queryParams", c.Request().URI().QueryArgs().String()))
			case "body":
				fields = append(fields, zap.ByteString("body", c.Body()))
			case "route":
				fields = append(fields, zap.String("route", c.Route().Path))
			case "method":
				fields = append(fields, zap.String("method", c.Method()))
			case "bytesReceived":
				fields = append(fields, zap.Int("bytesReceived", len(c.Request().Body())))
			}
		}
		cfg.Logger.Info("Request", fields...)
		// Handle request, store err for logging
		chainErr := c.Next()
		fields = fields[:0]
		// Manually call error handler
		if chainErr != nil {
			if err := errHandler(c, chainErr); err != nil {
				_ = c.SendStatus(fiber.StatusInternalServerError)
			}
		}

		// Set latency stop time
		if latencyEnabled {
			stop = time.Now()
		}
		cost := stop.Sub(start)
		for _, field := range cfg.Fields {
			switch field {
			case "latency":
				fields = append(fields, zap.String("latency", cost.String()))
			case "status":
				fields = append(fields, zap.Int("status", c.Response().StatusCode()))
			case "resBody":
				if strings.Contains(string(c.Response().Header.ContentType()), "application/json") {
					fields = append(fields, zap.ByteString("resBody", c.Response().Body()))
				}
			case "bytesSent":
				fields = append(fields, zap.Int("bytesSent", len(c.Response().Body())))
			case "error":
				if chainErr != nil {
					fields = append(fields, zap.String("error", chainErr.Error()))
				}
			}
		}

		// Return fields by status code
		s := c.Response().StatusCode()

		switch {
		case s >= 500:
			cfg.Logger.With(zap.Error(err)).Error(cfg.Messages[0], fields...)
		case s >= 400:
			cfg.Logger.With(zap.Error(err)).Warn(cfg.Messages[1], fields...)
		default:
			cfg.Logger.Info(cfg.Messages[2], fields...)
		}

		return nil
	}
}

func contains(needle string, slice []string) bool {
	for _, e := range slice {
		if e == needle {
			return true
		}
	}
	return false
}

func getUsername(ctx *fiber.Ctx) *infra.Authentication {
	userCtx := ctx.Locals("auth")
	if userCtx != nil {
		return userCtx.(*infra.Authentication)
	}
	return nil
}
