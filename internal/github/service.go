package github

import (
	"context"
	"net/http"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v42/github"
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

	return &Service{
		atr: atr,
	}, nil
}

func (s *Service) ListInstallations(ctx context.Context) ([]*github.Installation, error) {
	res, _, err := github.NewClient(&http.Client{Transport: s.atr}).Apps.ListInstallations(ctx, &github.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to list installations")
	}
	return res, nil
}
