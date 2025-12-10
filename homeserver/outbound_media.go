package homeserver

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
)

// DownloadMedia - Implements `media.Downloader`
func (h *Homeserver) DownloadMedia(ctx context.Context, origin string, mediaId string) ([]byte, error) {
	// TODO: Replace Client-Server API call with something a bit more sophisticated
	path, err := url.JoinPath(h.mediaClientUrl, fmt.Sprintf("/_matrix/client/v1/media/download/%s/%s", url.PathEscape(origin), url.PathEscape(mediaId)))
	if err != nil {
		return nil, err
	}
	log.Printf("Downloading media: %s", path)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", h.mediaClientAccessToken))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d", res.StatusCode)
	}

	b, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	return b, nil
}
