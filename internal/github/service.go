package github

import (
	"net/http"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/pkg/errors"
)

type Service struct {
	atr *ghinstallation.AppsTransport
}

type Config struct {
	AppID         int64  `mapstructure:"appID"`
	PrivateKeyPem string `mapstructure:"privateKeyPem"`
}

func NewService(conf Config) (*Service, error) {
	atr, err := ghinstallation.NewAppsTransport(http.DefaultTransport, conf.AppID, []byte(conf.PrivateKeyPem))
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create ghinstallation.AppsTransport")
	}

	ghinstallation.NewFromAppsTransport(atr, 0)

	return &Service{
		atr: atr,
	}, nil
}
