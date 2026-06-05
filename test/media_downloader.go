package test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var SleepFor60SecondsOnDownload = []byte("plz sleep 60s")

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

	b, ok := s.Media[origin+"/"+mediaId]
	if ok && string(b) == string(SleepFor60SecondsOnDownload) {
		select {
		case <-time.After(60 * time.Second):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if ok {
		return b, nil
	}

	return nil, errors.New("media not found")
}
