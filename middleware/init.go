package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/etag"
	"github.com/gofiber/fiber/v2/middleware/monitor"
	"github.com/gofiber/fiber/v2/middleware/pprof"
	"golang-ast/conf"

	rcp "github.com/gofiber/fiber/v2/middleware/recover"
	"go.uber.org/zap"
)

func Use(server *fiber.App, logger *zap.Logger, authCfg *conf.AuthConfig) *zap.Logger {
	server.Use(rcp.New())
	server.Use(cors.New(cors.Config{
		AllowOrigins:     "*",
		AllowHeaders:     "Authorization, Origin, X-Requested-With, Content-Type, Accept",
		AllowCredentials: true,
	}))
	server.Use(pprof.New())
	server.Use(etag.New(etag.Config{Weak: true}))
	server.Use(NewFiberLog(LogConfig{
		Next:     nil,
		Logger:   logger,
		Fields:   []string{"ips", "port", "url", "method", "status", "latency", "queryParams", "body", "resBody", "error"},
		Messages: []string{"Server error", "Client error", "Success"},
		OpLogCfg: authCfg,
	}))

	server.Use(NewAuthFilter())
	server.Get("/monitor", monitor.New())

	return logger
}
