package v1

import (
	"net/http"

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

	g.GET("/repositories", func(c echo.Context) error {
		res, err := svc.FullScan(c.Request().Context())
		if err != nil {
			l.WithError(err).Error("Failed to scan repositories")

			return c.String(500, err.Error())
		}
		return c.JSON(200, res)
	})

	g.POST("/webhook", func(c echo.Context) error {
		err := svc.ScheduleImageUpdates(c.Request().Context())
		if err != nil {
			l.WithError(err).Error("Failed to schedule image updates")

			return c.String(500, err.Error())
		}
		return c.NoContent(http.StatusAccepted)
	})
}
