package test

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMediaDownloader(t *testing.T) {
	t.Parallel()

	// Ensures that the *testing* media downloader works as expected

	downloader := MustMakeMediaDownloader(t)

	b, err := downloader.DownloadMedia(context.Background(), "example.org", "abc123")
	assert.ErrorContains(t, err, "media not found")
	assert.Nil(t, b)

	downloader2 := downloader.Set("example.org", "abc123", []byte("test")) // should be chainable
	assert.Equal(t, downloader, downloader2)

	b, err = downloader.DownloadMedia(context.Background(), "example.org", "abc123")
	assert.NoError(t, err)
	assert.Equal(t, []byte("test"), b)
}

func TestMediaDownloaderSleepsOnRequest(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		start := time.Now()
		downloader := MustMakeMediaDownloader(t)

		downloader.Set("example.org", "abc123", SleepFor60SecondsOnDownload)

		b, err := downloader.DownloadMedia(context.Background(), "example.org", "abc123")
		assert.NoError(t, err)
		assert.Equal(t, SleepFor60SecondsOnDownload, b)
		assert.True(t, time.Since(start) >= 60*time.Second)
	})
}
