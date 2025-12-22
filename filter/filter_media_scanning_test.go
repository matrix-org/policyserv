package filter

import (
	"context"
	"testing"

	"github.com/matrix-org/gomatrixserverlib"
	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/content"
	"github.com/matrix-org/policyserv/filter/classification"
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
			EnabledNames:           []string{MediaScanningFilterName},
			MinimumSpamVectorValue: 0.0,
			MaximumSpamVectorValue: 1.0,
		}},
	}
	memStorage := test.NewMemoryStorage(t)
	defer memStorage.Close()
	ps := test.NewMemoryPubsub(t)
	defer ps.Close()
	scanner := test.NewMemoryContentScanner(t)
	set, err := NewSet(cnf, memStorage, ps, test.MustMakeAuditQueue(5), scanner)
	assert.NoError(t, err)
	assert.NotNil(t, set)

	spammyBytes := []byte("this is spam")
	neutralBytes := []byte("this is neutral")

	// Note: we set the CSAM classification in both expectations so we can detect that the filter actually ran the scanner.
	// Only the first will result in a spam response though because it sets the spam classification.
	scanner.Expect(content.TypePhoto, spammyBytes, []classification.Classification{classification.CSAM, classification.Spam}, nil)
	scanner.Expect(content.TypePhoto, neutralBytes, []classification.Classification{classification.CSAM}, nil)

	spammyMxcUri := "mxc://example.org/spam"
	neutralMxcUri := "mxc://example.org/neutral"

	downloader := test.MustMakeMediaDownloader(t).
		Set("example.org", "spam", spammyBytes).
		Set("example.org", "neutral", neutralBytes)

	spammyEvent1 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$spam1",
		RoomId:  "!foo:example.org",
		Type:    "org.example.the_event_type_doesnt_matter_in_this_test",
		Content: map[string]any{
			"url": spammyMxcUri,
		},
	})
	spammyEvent2 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$spam2",
		RoomId:  "!foo:example.org",
		Type:    "org.example.the_event_type_doesnt_matter_in_this_test",
		Content: map[string]any{
			"info": map[string]any{
				"thumbnail_url": spammyMxcUri,
			},
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
				"thumbnail_url": neutralMxcUri,
			},
		},
	})

	assertSpamVector := func(event gomatrixserverlib.PDU, isSpam bool) {
		before := downloader.DownloadCalls
		vecs, err := set.CheckEvent(context.Background(), event, downloader)
		assert.NoError(t, err)
		assert.Equal(t, before+1, downloader.DownloadCalls)
		assert.Equal(t, 1.0, vecs.GetVector(classification.CSAM)) // always set regardless of spam/neutral
		if isSpam {
			assert.Equal(t, 1.0, vecs.GetVector(classification.Spam))
		} else {
			// Because the filter doesn't flag things as "not spam", the seed value should survive
			assert.Equal(t, 0.5, vecs.GetVector(classification.Spam))
		}
	}
	assertSpamVector(spammyEvent1, true)
	assertSpamVector(spammyEvent2, true)
	assertSpamVector(neutralEvent1, false)
	assertSpamVector(neutralEvent2, false)
}

func TestMediaScanningFilterClassifiesAsUnsafeOnScanError(t *testing.T) {
	t.Parallel()
	t.Skip("not implemented yet")
}
