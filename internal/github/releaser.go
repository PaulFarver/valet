package github

import (
	"context"
	"fmt"
	"net/http"
	"path"

	"github.com/google/go-github/v42/github"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

type ReleaserConfig struct {
	Rules []Rule `yaml:"rules"`
}

type Rule struct{}

var ErrFileMissing = errors.New("File missing in repository")

type Releaser struct {
	Client     *github.Client
	Repository *github.Repository
	Config     ReleaserConfig
}

func (s *Service) NewReleaser(ctx context.Context, client *github.Client, repo *github.Repository) (*Releaser, error) {
	file := s.config.ReleaseConfigPath
	r, o, err := client.Repositories.DownloadContents(ctx, repo.GetOwner().GetLogin(), repo.GetName(), file, &github.RepositoryContentGetOptions{
		Ref: repo.GetDefaultBranch(),
	})
	if o.StatusCode == http.StatusNotFound {
		return nil, ErrFileMissing
	}
	if errors.Is(err, fmt.Errorf("No file named %s found in %s", path.Base(file), path.Dir(file))) {
		return nil, ErrFileMissing
	}
	defer r.Close()

	var config ReleaserConfig
	if err := yaml.NewDecoder(r).Decode(&config); err != nil {
		return nil, errors.Wrap(err, "Failed to decode config")
	}
	// TODO: validate config

	return &Releaser{
		Client:     client,
		Repository: repo,
		Config:     config,
	}, nil
}

func (r *Releaser) ScanAndUpdate(ctx context.Context) error {
	ref, _, err := r.Client.Git.GetRef(ctx, r.Repository.GetOwner().GetLogin(), r.Repository.GetName(), "heads/master")
	if err != nil {
		return errors.Wrap(err, "Failed to get ref")
	}

	tree, _, err := r.Client.Git.GetTree(ctx, r.Repository.Owner.GetLogin(), r.Repository.GetName(), ref.Object.GetSHA(), true)
	if err != nil {
		return errors.Wrap(err, "Failed to get tree")
	}

	for _, entry := range tree.Entries {
		if entry.GetType() == "blob" {
			logrus.Info(entry.GetPath())
		}
	}
	return nil
}
