package filter

import (
	"context"
	"errors"
	"io"
	"log"
	"regexp"
	"strconv"
	"strings"

	"github.com/matrix-org/policyserv/event"
	"github.com/matrix-org/policyserv/harms"
	"github.com/matrix-org/policyserv/internal"
	"golang.org/x/net/html"
)

// Developer note: this filter is force-enabled and cannot be disabled.

const InlineEmojiSizeFilterName = "InlineEmojiSizeFilter"

func init() {
	mustRegister(InlineEmojiSizeFilterName, &InlineEmojiSizeFilter{})
}

type InlineEmojiSizeFilter struct {
}

func (f *InlineEmojiSizeFilter) MakeFor(set *Set) (Instanced, error) {
	return &InstancedInlineEmojiSizeFilter{
		set:             set,
		maxHeightPixels: internal.Dereference(set.communityConfig.InlineEmojiSizeFilterMaxHeightPixels),
	}, nil
}

var sizeRegex = regexp.MustCompile(`^\d+$`) // Example: "100" (pixels). This also prevents negative sizes.

type InstancedInlineEmojiSizeFilter struct {
	set             *Set
	maxHeightPixels int

	// Used by tests to create error conditions
	tokenizerCallback func(tokenizer *html.Tokenizer)
}

func (i *InstancedInlineEmojiSizeFilter) Name() string {
	return InlineEmojiSizeFilterName
}

func (i *InstancedInlineEmojiSizeFilter) CheckEvent(ctx context.Context, input *EventInput) (*harms.ContentInfo, error) {
	htmlRepresentations, err := event.ExtractHtmlRepresentations(input.Event)
	if err != nil {
		return nil, err
	}

	for _, htmlRepresentation := range htmlRepresentations {
		// Find all image tags so we can check their spec compliance.
		// As of writing, no released spec exists for inline emoji.
		// See https://github.com/matrix-org/matrix-spec-proposals/blob/9759751d4e0166edfa3b411924208742d8987c89/proposals/2545-emotes.md#sending
		parser := html.NewTokenizer(strings.NewReader(htmlRepresentation))
		if i.tokenizerCallback != nil {
			i.tokenizerCallback(parser)
		}
		for {
			tokenType := parser.Next()
			if tokenType == html.ErrorToken {
				err = parser.Err()
				if errors.Is(err, io.EOF) {
					break
				}
				return nil, err
			}

			if tokenType == html.StartTagToken || tokenType == html.SelfClosingTagToken {
				name, _ := parser.TagName()
				if string(name) != "img" {
					continue
				}

				// The `data-mx-emoticon` attribute MUST be present to be considered an emoji. Otherwise, it's
				// just an inline image. This filter denies inline images even if they're allowed by other filters.

				// The `height` attribute MUST also be present. It's required to be an integer value, so we validate
				// that. Note that the MSC previously recommended that the `height` be set to "32px", meaning some
				// events would not be valid under this filter - this is expected. Browsers/clients tend to ignore
				// invalid `height` attributes (ie: non-integer values), meaning the unbounded height could still
				// apply.

				isEmoticon := false
				alreadySawHeight := false
				for {
					key, val, moreAttrs := parser.TagAttr()
					if strings.ToLower(string(key)) == "data-mx-emoticon" {
						isEmoticon = true // we don't care about the value (if any)
					}
					if strings.ToLower(string(key)) == "height" {
						if alreadySawHeight {
							log.Printf("[%s | %s] Multiple height attributes found on inline image", input.Event.EventID(), input.Event.RoomID().String())
							return harms.ProhibitedContent(harms.SpamGeneral, harms.PolicyservSpecNonCompliance), nil
						}
						alreadySawHeight = true

						size := strings.ToLower(string(val))
						if !sizeRegex.MatchString(size) {
							// We don't log the size we saw because it's potentially spammy
							log.Printf("[%s | %s] Invalid height attribute on inline image", input.Event.EventID(), input.Event.RoomID().String())
							return harms.ProhibitedContent(harms.SpamGeneral, harms.PolicyservSpecNonCompliance), nil
						}

						// Parse the integer
						height, err := strconv.Atoi(size)
						if err != nil {
							// Don't log the size we saw because it's potentially spammy
							log.Printf("[%s | %s] Failed to parse height attribute on inline image", input.Event.EventID(), input.Event.RoomID().String())
							return harms.ProhibitedContent(harms.SpamGeneral, harms.PolicyservSpecNonCompliance), nil
						}

						if height > i.maxHeightPixels {
							// Don't log the size we saw because it's potentially spammy
							log.Printf("[%s | %s] Height attribute on inline image is too large", input.Event.EventID(), input.Event.RoomID().String())
							return harms.ProhibitedContent(harms.SpamGeneral, harms.PolicyservSpecNonCompliance), nil
						}
					}
					if !moreAttrs {
						break
					}
				}

				// If the tag isn't an emoji or we didn't see a height attribute, flag as spam
				if !isEmoticon || !alreadySawHeight {
					return harms.ProhibitedContent(harms.SpamGeneral, harms.PolicyservSpecNonCompliance), nil
				}
			}
		}
	}

	return harms.NeutralContent(), nil
}
