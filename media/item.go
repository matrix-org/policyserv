package media

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"
)

var InvalidMediaUrlError = errors.New("invalid media url")

// Item - A piece of media that was extracted from an event.
type Item struct {
	Origin  string
	MediaId string

	downloader Downloader
}

// NewItem - Creates a new Item instance from the given URL. Returns an InvalidMediaUrlError if the URL is invalid
// or not an MXC URL.
func NewItem(mediaUrl string, downloader Downloader) (*Item, error) {
	parsed, err := url.Parse(mediaUrl)
	if err != nil {
		return nil, errors.Join(InvalidMediaUrlError, err)
	}
	if parsed.Scheme != "mxc" {
		return nil, errors.Join(InvalidMediaUrlError, errors.New("not an mxc uri"))
	}

	return &Item{
		Origin:     parsed.Host,
		MediaId:    parsed.Path[1:], // strip leading slash,
		downloader: downloader,
	}, nil
}

func (m *Item) Download() ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second) // 30s is somewhat arbitrary - we just don't want to wait forever
	defer cancel()
	return m.downloader.DownloadMedia(ctx, m.Origin, m.MediaId)
}

func (m *Item) String() string {
	return fmt.Sprintf("mxc://%s/%s", m.Origin, m.MediaId)
}
