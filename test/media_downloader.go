package test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

type StaticMediaDownloader struct {
	T             *testing.T
	Media         map[string][]byte
	DownloadCalls int
}

func MustMakeMediaDownloader(t *testing.T) *StaticMediaDownloader {
	return &StaticMediaDownloader{
		T:     t,
		Media: make(map[string][]byte),
	}
}

func (s *StaticMediaDownloader) Set(origin string, mediaId string, bytes []byte) *StaticMediaDownloader {
	s.Media[origin+"/"+mediaId] = bytes
	return s
}

func (s *StaticMediaDownloader) DownloadMedia(ctx context.Context, origin string, mediaId string) ([]byte, error) {
	assert.NotNil(s.T, ctx, "context is required")

	s.DownloadCalls++

	if bytes, ok := s.Media[origin+"/"+mediaId]; ok {
		return bytes, nil
	}

	return nil, errors.New("media not found")
}
