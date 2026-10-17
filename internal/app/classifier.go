package app

import (
	"context"

	"github.com/lookinalx/fiona/ml"
)

// Classifier tags images with ML categories.
type Classifier interface {
	Classify(ctx context.Context, paths []string) (map[string]string, error)
	Close() error
}

// sidecar starts fiona-classifier next to the fiona binary on first use.
type sidecar struct {
	configPath string
	server     *ml.Server
	client     *ml.Client
}

// NewSidecar returns a classifier backed by the fiona-classifier process.
// Close stops the process.
func NewSidecar(configPath string) Classifier {
	return &sidecar{configPath: configPath}
}

func (s *sidecar) Classify(ctx context.Context, paths []string) (map[string]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.server == nil {
		server := ml.NewServer(s.configPath)
		if err := server.Start(); err != nil {
			server.Stop()
			return nil, err
		}
		s.server = server
		s.client = ml.NewClient(server.BaseURL())
	}
	return s.client.Classify(paths)
}

func (s *sidecar) Close() error {
	if s.server == nil {
		return nil
	}
	err := s.server.Stop()
	s.server = nil
	return err
}

// Client wraps an already running classifier, like a test server.
func Client(baseURL string) Classifier {
	return clientClassifier{client: ml.NewClient(baseURL)}
}

type clientClassifier struct {
	client *ml.Client
}

func (c clientClassifier) Classify(_ context.Context, paths []string) (map[string]string, error) {
	return c.client.Classify(paths)
}

func (c clientClassifier) Close() error { return nil }
