package chart

import (
	"context"
	_ "embed"
	"errors"

	"gopkg.in/yaml.v3"
)

type IndexResponse struct {
	APIVersion APIVersion             `yaml:"apiVersion"`
	Entries    map[string][]ChartInfo `yaml:"entries"`
	Generated  string                 `yaml:"generated"`
}

type ChartInfo struct {
	APIVersion   APIVersion   `yaml:"apiVersion"`
	AppVersion   *string      `yaml:"appVersion,omitempty"`
	Created      string       `yaml:"created"`
	Description  string       `yaml:"description"`
	Digest       string       `yaml:"digest"`
	Home         *string      `yaml:"home,omitempty"`
	Icon         *string      `yaml:"icon,omitempty"`
	Maintainers  []Maintainer `yaml:"maintainers"`
	Name         string       `yaml:"name"`
	Sources      []string     `yaml:"sources"`
	Urls         []string     `yaml:"urls"`
	Version      string       `yaml:"version"`
	Deprecated   *bool        `yaml:"deprecated,omitempty"`
	Keywords     []string     `yaml:"keywords"`
	KubeVersion  *string      `yaml:"kubeVersion,omitempty"`
	Dependencies []Dependency `yaml:"dependencies"`
}

type Dependency struct {
	Name       string  `yaml:"name"`
	Repository string  `yaml:"repository"`
	Version    string  `yaml:"version"`
	Alias      *string `yaml:"alias,omitempty"`
}

type Maintainer struct {
	Email *string `yaml:"email"`
	Name  string  `yaml:"name"`
	URL   *string `yaml:"url,omitempty"`
}

type APIVersion string

const (
	V1 APIVersion = "v1"
	V2 APIVersion = "v2"
)

type Service interface {
	ListVersions(ctx context.Context, repository, chart string) ([]string, error)
}

type ServiceMock struct {
	StaticIndexResponse IndexResponse
}

//go:embed mock.yaml
var resp []byte

func NewServiceMock() Service {
	indexResponse := IndexResponse{}

	yaml.Unmarshal(resp, &indexResponse)

	return &ServiceMock{
		StaticIndexResponse: indexResponse,
	}
}

func (s *ServiceMock) ListVersions(ctx context.Context, repository, chart string) ([]string, error) {
	c, ok := s.StaticIndexResponse.Entries[chart]
	if !ok {
		return nil, errors.New("chart not found")
	}

	versions := make([]string, len(c))
	for i, info := range c {
		versions[i] = info.Version
	}
	return versions, nil
}
