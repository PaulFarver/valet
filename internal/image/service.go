package image

import "context"

type Service interface {
	ListTags(ctx context.Context, repository string) ([]string, error)
}

type ServiceMock struct{}

func NewServiceMock() Service {
	return &ServiceMock{}
}

func (s *ServiceMock) ListTags(ctx context.Context, repository string) ([]string, error) {
	return []string{"1.0.0", "1.0.1"}, nil
}
