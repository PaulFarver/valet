package github

import (
	"context"
	"net/http"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v42/github"
	"github.com/paulfarver/valet/internal/chart"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type Service struct {
	atr          *ghinstallation.AppsTransport
	config       Config
	chartService chart.Service
}

type Config struct {
	AppID             int64  `mapstructure:"appID"`
	PrivateKeyPem     string `mapstructure:"privateKeyPem"`
	ReleaseConfigPath string `mapstructure:"releaseConfig"`
}

func NewService(conf Config, chartService chart.Service) (*Service, error) {
	atr, err := ghinstallation.NewAppsTransport(http.DefaultTransport, conf.AppID, []byte(conf.PrivateKeyPem))
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create ghinstallation.AppsTransport")
	}

	return &Service{
		atr:          atr,
		config:       conf,
		chartService: chartService,
	}, nil
}

func (s *Service) ListInstallations(ctx context.Context) ([]*github.Installation, error) {
	res, _, err := github.NewClient(&http.Client{Transport: s.atr}).Apps.ListInstallations(ctx, &github.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to list installations")
	}

	return res, nil
}

func (s *Service) FullScan(ctx context.Context) ([]*github.Repository, error) {
	installations, err := s.ListInstallations(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to list installations during full scan")
	}

	repositories := []*github.Repository{}
	for _, installation := range installations {
		transport := ghinstallation.NewFromAppsTransport(s.atr, installation.GetID())

		client := github.NewClient(&http.Client{Transport: transport})
		repos, err := s.ScanInstallation(ctx, client)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to scan installation")
		}
		repositories = append(repositories, repos...)
	}
	return repositories, nil
}

func (s *Service) GetReleasers(ctx context.Context, installation *github.Installation, l logrus.FieldLogger) ([]*Releaser, error) {
	transport := ghinstallation.NewFromAppsTransport(s.atr, installation.GetID())
	client := github.NewClient(&http.Client{Transport: transport})
	response, _, err := client.Apps.ListRepos(ctx, nil)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to list repositories")
	}
	releasers := []*Releaser{}
	for _, repo := range response.Repositories {
		releaser, err := s.NewReleaser(ctx, client, repo, l, s.chartService)
		if err != nil {
			l.WithError(err).WithField("repo", repo.GetFullName()).Warn("Failed to create releaser")
			continue
		}
		releasers = append(releasers, releaser)
	}
	return releasers, nil
}

func (s *Service) ScanInstallation(ctx context.Context, client *github.Client) ([]*github.Repository, error) {
	repos, _, err := client.Apps.ListRepos(ctx, nil)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to list repos")
	}

	return repos.Repositories, nil
}

func (s *Service) ScheduleImageUpdates(ctx context.Context, l logrus.FieldLogger) error {
	installations, err := s.ListInstallations(ctx)
	if err != nil {
		return errors.Wrap(err, "Failed to list installations")
	}

	// TODO: dispatch to a worker pool
	releasers := []*Releaser{}
	for _, installation := range installations {
		rel, err := s.GetReleasers(ctx, installation, l)
		if err != nil {
			l.WithError(err).WithField("installation", installation.GetID()).Warn("Failed to create releasers")
			continue
		}
		releasers = append(releasers, rel...)
	}

	for _, releaser := range releasers {
		releaser.ScanAndUpdate(ctx)
	}

	return nil
}
