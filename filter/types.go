package filter

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/matrix-org/policyserv/filter/classification"
	"github.com/matrix-org/policyserv/filter/confidence"
	"github.com/matrix-org/policyserv/media"
	"github.com/matrix-org/gomatrixserverlib"
)

var InvalidMediaUrlError = errors.New("invalid media url")

// Media - A piece of media that was extracted from an event.
type Media struct {
	Origin  string
	MediaId string

	downloader media.Downloader
}

// NewMedia - Creates a new Media instance from the given URL. Returns an InvalidMediaUrlError if the URL is invalid
// or not an MXC URL.
func NewMedia(mediaUrl string, downloader media.Downloader) (*Media, error) {
	parsed, err := url.Parse(mediaUrl)
	if err != nil {
		return nil, errors.Join(InvalidMediaUrlError, err)
	}
	if parsed.Scheme != "mxc" {
		return nil, errors.Join(InvalidMediaUrlError, errors.New("not an mxc uri"))
	}

	return &Media{
		Origin:     parsed.Host,
		MediaId:    parsed.Path[1:], // strip leading slash,
		downloader: downloader,
	}, nil
}

func (m *Media) Download() ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second) // 30s is somewhat arbitrary - we just don't want to wait forever
	defer cancel()
	return m.downloader.DownloadMedia(ctx, m.Origin, m.MediaId)
}

func (m *Media) String() string {
	return fmt.Sprintf("mxc://%s/%s", m.Origin, m.MediaId)
}

// Input - A filter input.
type Input struct {
	// The event to process/check.
	Event gomatrixserverlib.PDU

	// The confidence.Vectors so far. Note that the first set group will receive a classification.Spam vector of 0.5
	IncrementalConfidenceVectors confidence.Vectors

	// Extracted media items from the event.
	Medias []*Media

	// The context used for auditing the performance of policyserv's filters.
	auditContext *auditContext
}

// Instanced - A Set-specific filter.
type Instanced interface {
	// Name - The name of the filter for logging and metrics.
	Name() string

	// CheckEvent - Processes the given event, returning classifications about it. If an error occurred, the classifications
	// array will be nil/empty.
	CheckEvent(ctx context.Context, input *Input) ([]classification.Classification, error)
}

// CanBeInstanced - The base filter type, registered at compile/run time and used by Sets to create a long-lived
// Instanced instance.
type CanBeInstanced interface {
	// MakeFor - Creates a long-lived Instanced for the provided Set. If an error occurred, the Instanced will
	// be nil.
	MakeFor(set *Set) (Instanced, error)
}
