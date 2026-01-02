package filter

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/matrix-org/gomatrixserverlib"
	"github.com/matrix-org/policyserv/content"
	"github.com/matrix-org/policyserv/filter/classification"
	"github.com/matrix-org/policyserv/media"
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

func (f *InstancedMediaScanningFilter) CheckEvent(ctx context.Context, input *Input) ([]classification.Classification, error) {
	if len(input.Medias) == 0 {
		return nil, nil // return early to avoid doing work
	}

	retChans := make([]chan []classification.Classification, len(input.Medias))

	// Schedule the channel cleanup in case something goes wrong while we're creating them
	defer func() {
		for _, ch := range retChans {
			close(ch)
		}
	}()

	// Create the channels & async the scanning work
	for i, m := range input.Medias {
		ch := make(chan []classification.Classification, 1)
		retChans[i] = ch
		go f.scanMedia(ctx, input.Event, m, ch)
	}

	// We don't want to wait forever for the scanning to complete, so set a timeout
	readTimeout, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Collect all of the scan results, up to a timeout
	classifications := make([]classification.Classification, 0)
	for _, ch := range retChans {
		select {
		case <-readTimeout.Done():
			return nil, errors.New("timed out waiting for media scanning results")
		case subClassifications := <-ch:
			if subClassifications != nil {
				classifications = append(classifications, subClassifications...)
			}
		}
	}

	// Return those results
	return classifications, nil
}

func (f *InstancedMediaScanningFilter) scanMedia(ctx context.Context, event gomatrixserverlib.PDU, media *media.Item, ch chan<- []classification.Classification) {
	log.Printf("[%s | %s] Downloading media %s", event.EventID(), event.RoomID().String(), media)
	b, err := media.Download()
	if err != nil {
		log.Printf("[%s | %s] Error downloading media: %s", event.EventID(), event.RoomID().String(), err)
		ch <- []classification.Classification{classification.Spam, classification.Unsafe} // Consider errors to be spam for now.
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
	}

	log.Printf("[%s | %s] Media scan result on %s: %v", event.EventID(), event.RoomID().String(), media, res)

	ch <- res
}
