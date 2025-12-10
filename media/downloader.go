package media

import "context"

type Downloader interface {
	DownloadMedia(ctx context.Context, origin string, mediaId string) ([]byte, error)
}
