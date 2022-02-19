package v1

import (
	"github.com/labstack/echo/v4"
	"github.com/paulfarver/valet/internal/github"
	"github.com/sirupsen/logrus"
)

type Handler struct{}

func Register(g *echo.Group, l *logrus.Logger, svc *github.Service) {
	// h := &Handler{}

	g.GET("/status", func(c echo.Context) error {
		return c.String(200, "ok")
	})

	g.GET("/list", func(c echo.Context) error {
		res, err := svc.ListInstallations(c.Request().Context())
		if err != nil {
			l.WithError(err).Error("Failed to list installations")

			return c.String(500, err.Error())
		}
		return c.JSON(200, res)
	})
}
