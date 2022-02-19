package rest

import (
	"context"
	"fmt"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/paulfarver/valet/internal/github"
	v1 "github.com/paulfarver/valet/internal/rest/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type Config struct {
	Port int `mapstructure:"port"`
}

type Server struct {
	echo   *echo.Echo
	config Config
}

func NewServer(conf Config, logger *logrus.Logger, svc *github.Service) (*Server, error) {
	e := echo.New()

	e.HideBanner = true
	e.HidePort = true

	// prometheusMiddleware, err := xmid.PrometheusWithConfig(xmid.PrometheusConfig{
	// 	Registerer: prom,
	// })
	// if err != nil {
	// 	return nil, err
	// }

	e.Use(
		middleware.Recover(),
		// 	prometheusMiddleware,
		middleware.CORSWithConfig(middleware.CORSConfig{
			AllowOrigins: []string{"*"},
		}),
	// 	xmid.RequestIDWithConfig(xmid.RequestIDConfig{
	// 		Generator: middleware.DefaultRequestIDConfig.Generator,
	// 		Header:    "X-Trace-ID",
	// 		Skipper:   middleware.DefaultRequestIDConfig.Skipper,
	// 	}),
	// 	middleware.GzipWithConfig(middleware.GzipConfig{
	// 		Level:   gzipCompressionLevel,
	// 		Skipper: middleware.DefaultGzipConfig.Skipper,
	// 	}),
	)

	root := e.Group("/v1")
	// root := e.Group("/books")

	v1.Register(root, logger, svc)

	return &Server{
		echo:   e,
		config: conf,
	}, nil
}

func (s *Server) Start() error {
	return errors.Wrap(s.echo.Start(fmt.Sprintf(":%d", s.config.Port)), "Failed to start server")
}

func (s *Server) Shutdown(ctx context.Context) error {
	return errors.Wrap(s.echo.Shutdown(ctx), "Failed to shutdown server")
}
