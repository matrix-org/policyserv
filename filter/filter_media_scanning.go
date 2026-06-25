package filter

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/matrix-org/gomatrixserverlib"
	"github.com/matrix-org/policyserv/content"
	"github.com/matrix-org/policyserv/harms"
	"github.com/matrix-org/policyserv/media"
	"github.com/matrix-org/policyserv/storage"
)

const MediaScanningFilterName = "MediaScanningFilter"

func init() {
	mustRegister(MediaScanningFilterName, &MediaScanningFilter{})
}

type MediaScanningFilter struct {
}

func (m *MediaScanningFilter) MakeFor(set *Set) (Instanced, error) {
	scanner := set.contentScanner
	if scanner == nil {
		// "should never happen" because the community manager should only add this filter if there is a scanner configured
		return nil, errors.New("no scanner configured")
	}
	return &InstancedMediaScanningFilter{
		set:     set,
		scanner: scanner,
	}, nil
}

type InstancedMediaScanningFilter struct {
	set     *Set
	scanner content.Scanner
}

func (f *InstancedMediaScanningFilter) Name() string {
	return MediaScanningFilterName
}

func (f *InstancedMediaScanningFilter) CheckEvent(ctx context.Context, input *EventInput) (*harms.ContentInfo, error) {
	if len(input.Medias) == 0 {
		return harms.NeutralContent(), nil // return early to avoid doing work
	}

	retChans := make([]chan *harms.ContentInfo, len(input.Medias))

	// Create the channels & async the scanning work
	for i, m := range input.Medias {
		ch := make(chan *harms.ContentInfo, 1)
		retChans[i] = ch
		// scanMedia will close the channel when it's done - we don't need to do it here
		go f.scanMedia(ctx, input.Event, m, ch)
	}

	// We don't want to wait forever for the scanning to complete, so set a timeout
	readTimeout, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Collect all of the scan results, up to a timeout
	contentClass := harms.ContentClassNeutral
	harmIds := make([]harms.Harm, 0)
	for _, ch := range retChans {
		select {
		case <-readTimeout.Done():
			log.Printf("[%s | %s] Media scanning timed out", input.Event.EventID(), input.Event.RoomID().String())
			return harms.ProhibitedContent(harms.OtherGeneral), nil
		case mediaInfo := <-ch:
			if mediaInfo != nil {
				if contentClass < mediaInfo.Class() {
					contentClass = mediaInfo.Class()
				}
				for _, h := range mediaInfo.Harms() {
					harmIds = append(harmIds, h)
				}
			}
		}
	}

	// Return those results
	return harms.NewContentInfo(contentClass, harmIds...), nil
}

func (f *InstancedMediaScanningFilter) scanMedia(ctx context.Context, event gomatrixserverlib.PDU, media *media.Item, ch chan<- *harms.ContentInfo) {
	cached, err := f.set.storage.GetMediaClassification(ctx, media.String(), f.set.communityId)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		log.Printf("[%s | %s] Non-fatal error getting cached media classification: %s", event.EventID(), event.RoomID().String(), err)
	}
	if err == nil {
		log.Printf("[%s | %s] Using cached media classification for %s (%v)", event.EventID(), event.RoomID().String(), media, cached.Classifications)
		ch <- cached.Classifications.ContentInfo
		return
	}

	log.Printf("[%s | %s] Downloading media %s", event.EventID(), event.RoomID().String(), media)
	b, err := media.Download()
	if err != nil {
		log.Printf("[%s | %s] Error downloading media: %s", event.EventID(), event.RoomID().String(), err)
		ch <- harms.ProhibitedContent(harms.OtherGeneral) // Consider errors to be spam for now.
		return
	}

	// figure out what we're about to scan, if we can
	mimeType := http.DetectContentType(b)
	contentType := content.TypePhoto // assume it's a photo by default
	if strings.HasPrefix(mimeType, "video/") {
		contentType = content.TypeVideo
	}

	log.Printf("[%s | %s] Scanning media (%s:%s) %s", event.EventID(), event.RoomID().String(), contentType, mimeType, media)
	res, err := f.scanner.Scan(ctx, contentType, b)
	if err != nil {
		log.Printf("[%s | %s] Error scanning media: %s", event.EventID(), event.RoomID().String(), err)
		ch <- harms.ProhibitedContent(harms.OtherGeneral) // Consider errors to be spam for now.
		return
	}

	log.Printf("[%s | %s] Media scan result on %s: %v", event.EventID(), event.RoomID().String(), media, res)

	err = f.set.storage.UpsertMediaClassification(ctx, &storage.StoredMediaClassification{
		MxcUri:      media.String(),
		CommunityId: f.set.communityId,
		Classifications: storage.StoredClassifications{
			ContentInfo: res,
		},
	})
	if err != nil {
		log.Printf("[%s | %s] Non-fatal error caching media classification: %s", event.EventID(), event.RoomID().String(), err)
	}

	err = ctx.Err()
	if err != nil && (errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)) {
		// don't try to send on what is about to be a closed channel
		return
	}
	ch <- res
}
