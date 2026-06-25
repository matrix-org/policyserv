package filter

import (
	"context"
	"testing"
	"testing/synctest"

	"github.com/matrix-org/gomatrixserverlib"
	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/content"
	"github.com/matrix-org/policyserv/harms"
	"github.com/matrix-org/policyserv/media"
	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
)

func TestMediaScanningFilter(t *testing.T) {
	t.Parallel()

	cnf := &SetConfig{
		CommunityConfig: &config.CommunityConfig{
			MediaFilterMediaTypes: &[]string{"m.sticker", "m.image"},
		},
		Groups: []*SetGroupConfig{{
			EnabledNames: []string{MediaScanningFilterName},
			RunOnClasses: []harms.ContentClass{harms.ContentClassNeutral}, // everything is neutral by default in the test
		}},
	}
	memStorage := test.NewMemoryStorage(t)
	defer memStorage.Close()
	ps := test.NewMemoryPubsub(t)
	defer ps.Close()
	scanner := test.NewMemoryContentScanner(t)
	set, err := NewSet(cnf, memStorage, ps, test.NewMatrixNotifier(t), scanner)
	assert.NoError(t, err)
	assert.NotNil(t, set)

	spammyBytes := []byte("this is spam")
	neutralBytes := []byte("this is neutral")

	// Note: we set the CSAM classification in both expectations so we can detect that the filter actually ran the scanner.
	// Only the first will result in a spam response though because it sets the spam classification.
	scanner.Expect(content.TypePhoto, spammyBytes, harms.ProhibitedContent(harms.ChildSafetyCSAM, harms.SpamGeneral), nil)
	scanner.Expect(content.TypePhoto, neutralBytes, harms.ProhibitedContent(harms.ChildSafetyCSAM), nil)

	spammyMxcUri1 := "mxc://example.org/spam1"
	spammyMxcUri2 := "mxc://example.org/spam2"
	neutralMxcUri := "mxc://example.org/neutral"

	downloader := test.MustMakeMediaDownloader(t).
		Set("example.org", "spam1", spammyBytes).
		Set("example.org", "spam2", spammyBytes).
		Set("example.org", "neutral", neutralBytes)

	spammyEvent1 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$spam1",
		RoomId:  "!foo:example.org",
		Type:    "org.example.the_event_type_doesnt_matter_in_this_test",
		Content: map[string]any{
			"url": spammyMxcUri1,
		},
	})
	spammyEvent2 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$spam2",
		RoomId:  "!foo:example.org",
		Type:    "org.example.the_event_type_doesnt_matter_in_this_test",
		Content: map[string]any{
			"info": map[string]any{
				"thumbnail_url": spammyMxcUri2,
			},
		},
	})
	spammyEvent3 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$spam3",
		RoomId:  "!foo:example.org",
		Type:    "org.example.the_event_type_doesnt_matter_in_this_test",
		Content: map[string]any{
			"url": spammyMxcUri2, // repeat the same MXC URI we've already seen to ensure caches work
		},
	})
	neutralEvent1 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$neutral1",
		RoomId:  "!foo:example.org",
		Type:    "org.example.the_event_type_doesnt_matter_in_this_test",
		Content: map[string]any{
			"url": neutralMxcUri,
		},
	})
	neutralEvent2 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$neutral2",
		RoomId:  "!foo:example.org",
		Type:    "org.example.the_event_type_doesnt_matter_in_this_test",
		Content: map[string]any{
			"info": map[string]any{
				"thumbnail_url": neutralMxcUri, // also should be cached
			},
		},
	})

	// We can't use AssertCheckEvent here because we need to give it a downloader to use.
	assertCheckEvent := func(event gomatrixserverlib.PDU, expected *harms.ContentInfo) {
		info, err := set.CheckEvent(context.Background(), event, downloader)
		assert.NoError(t, err)
		test.AssertEqualContentInfo(t, expected, info)
	}

	assertCheckEvent(spammyEvent1, harms.ProhibitedContent(harms.ChildSafetyCSAM, harms.SpamGeneral))
	assert.Equal(t, 1, downloader.DownloadCalls)
	assertCheckEvent(spammyEvent2, harms.ProhibitedContent(harms.ChildSafetyCSAM, harms.SpamGeneral))
	assert.Equal(t, 2, downloader.DownloadCalls) // +1 call
	assertCheckEvent(spammyEvent3, harms.ProhibitedContent(harms.ChildSafetyCSAM, harms.SpamGeneral))
	assert.Equal(t, 2, downloader.DownloadCalls) // should have already cached the result

	// We always return a CSAM harm for testing, even on neutral media

	assertCheckEvent(neutralEvent1, harms.ProhibitedContent(harms.ChildSafetyCSAM))
	assert.Equal(t, 3, downloader.DownloadCalls) // +1 call
	assertCheckEvent(neutralEvent2, harms.ProhibitedContent(harms.ChildSafetyCSAM))
	assert.Equal(t, 3, downloader.DownloadCalls) // should have already cached the result too
}

func TestMediaScanningFilterGracefullyHandlesDownloadTimeouts(t *testing.T) {
	t.Parallel()

	// We use synctest to manipulate time itself - see https://go.dev/blog/testing-time#time
	synctest.Test(t, func(t *testing.T) {
		// Dev note: ideally we'd test the full filter stack like we do in other tests, but because
		// we're using synctest and the stack creates goroutines, we instead create the filter manually.
		// synctest requires all goroutines to be stopped before the test ends, which we can't control.
		f := &InstancedMediaScanningFilter{
			set: &Set{
				storage: test.NewMemoryStorage(t),
			},
			scanner: test.NewMemoryContentScanner(t),
		}

		downloader := test.MustMakeMediaDownloader(t).
			Set("example.org", "media", test.SleepFor60SecondsOnDownload)

		event := test.MustMakePDU(&test.BaseClientEvent{
			EventId: "$spam1",
			RoomId:  "!foo:example.org",
			Type:    "org.example.the_event_type_doesnt_matter_in_this_test",
			Content: map[string]any{
				"url": `mxc://example.org/media`,
			},
		})

		mediaItem, err := media.NewItem("mxc://example.org/media", downloader)
		assert.NoError(t, err)
		assert.NotNil(t, mediaItem)
		info, err := f.CheckEvent(context.Background(), &EventInput{
			Event:  event,
			Medias: []*media.Item{mediaItem},
		})
		assert.NoError(t, err)
		test.AssertEqualContentInfo(t, harms.ProhibitedContent(harms.OtherGeneral), info)
		assert.Equal(t, 1, downloader.DownloadCalls)
		synctest.Wait()
	})
}

func TestMediaScanningFilterGracefullyHandlesScannerError(t *testing.T) {
	t.Parallel()

	cnf := &SetConfig{
		Groups: []*SetGroupConfig{{
			EnabledNames: []string{MediaScanningFilterName},
			RunOnClasses: []harms.ContentClass{harms.ContentClassNeutral},
		}},
	}
	memStorage := test.NewMemoryStorage(t)
	defer memStorage.Close()
	ps := test.NewMemoryPubsub(t)
	defer ps.Close()
	scanner := test.NewMemoryContentScanner(t)
	set, err := NewSet(cnf, memStorage, ps, test.NewMatrixNotifier(t), scanner)
	assert.NoError(t, err)

	mediaBytes := []byte("some media")
	mxcUri := "mxc://example.org/error"
	downloader := test.MustMakeMediaDownloader(t).Set("example.org", "error", mediaBytes)

	event := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$error1",
		RoomId:  "!foo:example.org",
		Type:    "m.room.message",
		Content: map[string]any{"url": mxcUri},
	})

	// Simulate scanner error
	scanner.Expect(content.TypePhoto, mediaBytes, nil, assert.AnError)

	info, err := set.CheckEvent(context.Background(), event, downloader)
	assert.NoError(t, err)

	// Failed scanning should present as prohibited content (with general harm)
	test.AssertEqualContentInfo(t, harms.ProhibitedContent(harms.OtherGeneral), info)

	// Ensure it didn't cache the nil result
	cached, err := memStorage.GetMediaClassification(context.Background(), mxcUri, "")
	assert.Error(t, err)
	assert.Nil(t, cached)
}
