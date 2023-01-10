package server

import (
	"crypto/tls"
	_ "embed"
	"github.com/gofiber/fiber/v2"
	jsoniter "github.com/json-iterator/go"
	"go.uber.org/zap"
	"golang-ast/conf"
	"golang-ast/db"
	"golang-ast/infra"
	"golang-ast/middleware"
)

var json = jsoniter.Config{
	EscapeHTML:             false,
	SortMapKeys:            true,
	ValidateJsonRawMessage: true,
}.Froze()

type AdminServer struct {
	app  *fiber.App
	cfg  *conf.GConfig
	auth *infra.Authorization
	log  *zap.Logger
	cert tls.Certificate
}

func NewServer(conf *conf.GConfig, logger *zap.Logger, dbms *db.DB) (*AdminServer, error) {
	engine := fiber.New(fiber.Config{
		CaseSensitive:     true,
		AppName:           "ADMIN-SERVER",
		ReduceMemoryUsage: true,
		JSONEncoder:       json.Marshal,
		JSONDecoder:       json.Unmarshal,
	})
	middleware.Use(engine, logger.Named("\u001B[33m[Engine]\u001B[0m"), conf.AuthCfg)
	infra.NewAuthorization(conf.AuthCfg, dbms, logger.Named("[AUTH]"))

	srv := &AdminServer{
		app:  engine,
		cfg:  conf,
		auth: infra.GetAuthHandler(),
		log:  logger.Named("\u001B[32m[Server]\u001B[0m"),
	}
	root := engine.Group("/api")
	srv.Register(root)
	return srv, nil
}

func (srv *AdminServer) StartHttpServer() {
	err := srv.app.Listen(srv.cfg.AppCfg.HttpAddr)
	if err != nil {
		srv.log.Error("start admin server http err:", zap.Error(err))
		return
	}
}

func (srv *AdminServer) Close() {
	srv.auth.Close()
	err := srv.app.Shutdown()
	if err != nil {
		srv.log.Error("stop admin server err:", zap.Error(err))
		return
	}
	srv.log.Info("\u001B[32m Storm admin server close complete\u001B[0m")
}

type ErrorResponse struct {
	FailedField string
	Rule        string
	ErrValue    interface{}
}
