package chart

import (
	"context"
	_ "embed"
	"errors"

	"gopkg.in/yaml.v3"
)

type IndexResponse struct {
	Entries map[string][]ChartInfo `yaml:"entries"`
}

type ChartInfo struct {
	Version string `yaml:"version"`
}

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
