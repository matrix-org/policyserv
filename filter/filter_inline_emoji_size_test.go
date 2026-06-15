package filter

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/filter/classification"
	"github.com/matrix-org/policyserv/internal"
	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/html"
)

const inlineEmojiSizeFilterMaxPixels = 32

type inlineEmojiSizeFilterTestCase struct {
	name     string
	html     string
	isSpammy bool
}

// We have a large number of test cases because we need to test all possible variations of an HTML string:
// 1. Messages with and without a `data-mx-emoticon` attribute
// 2. Messages with and without a `height` attribute
// 3. Messages with and without a `height` attribute using units (units aren't allowed)
// 4. Messages with and without a `height` attribute using positive values
// 5. Messages with the `data-mx-emoticon` attribute appearing before or after the `height` attribute
// 6. Messages using and not using self-closing tags
// 7. Messages using uppercase, lowercase, or mixed case
// 8. Messages with image tags at the start, end, or middle
// 9. Messages with multiple image tags (including mixing the above cases)
//
// To keep track of this, we create a test case table for each condition and combine them as needed.

var inlineEmojiSizeFilterTestCases = make([]inlineEmojiSizeFilterTestCase, 0)

func init() {
	// Start with an easy test case: no images to detect (so essentially just "wasted" CPU)
	inlineEmojiSizeFilterTestCases = append(inlineEmojiSizeFilterTestCases, inlineEmojiSizeFilterTestCase{
		name:     "no images",
		html:     "<p>this is a regular <strong>message</strong></p>",
		isSpammy: false,
	})

	// Later in this function we loop over each "component" to create the test cases themselves

	emojiComponents := []inlineEmojiSizeFilterTestCase{{
		name:     "no data-mx-emoticon attribute",
		html:     ``,
		isSpammy: true,
	}, {
		name:     "data-mx-emoticon attribute with no value",
		html:     `data-mx-emoticon`,
		isSpammy: false,
	}, {
		name:     "data-mx-emoticon attribute with empty value",
		html:     `data-mx-emoticon=""`,
		isSpammy: false,
	}, {
		name:     "data-mx-emoticon attribute with non-empty value",
		html:     `data-mx-emoticon="shouldn't matter'"`, // the value isn't read by anything, but it could be specified for some reason
		isSpammy: false,
	}}

	heightComponents := []inlineEmojiSizeFilterTestCase{{
		name:     "no height attribute",
		html:     ``,
		isSpammy: true,
	}, {
		name:     "height attribute with no value",
		html:     `height`,
		isSpammy: true,
	}, {
		name:     "height attribute with empty value",
		html:     `height=""`,
		isSpammy: true,
	}, {
		name:     "height attribute with invalid value",
		html:     `height="invalid"`,
		isSpammy: true,
	}, {
		name:     "height attribute with invalid units",
		html:     `height="16px"`,
		isSpammy: true,
	}, {
		name:     "height attribute with proper size",
		html:     `height="16"`,
		isSpammy: false, // size is under inlineEmojiSizeFilterMaxPixels
	}, {
		name:     "height attribute with negative size",
		html:     `height="-16"`,
		isSpammy: true,
	}, {
		name:     "height attribute with size too large",
		html:     `height="64"`,
		isSpammy: true, // size is above inlineEmojiSizeFilterMaxPixels
	}}

	// These next components don't affect whether a message is spammy, so we don't need to set isSpammy

	closingTagComponents := []inlineEmojiSizeFilterTestCase{{
		name: "self-closing tag",
		html: `/>`,
	}, {
		name: "closing tag",
		html: `></img>`,
	}, {
		name: "closing tag with content",
		html: `>content</img>`,
	}}

	prefixComponents := []inlineEmojiSizeFilterTestCase{{
		name: "no prefix",
		html: ``,
	}, {
		name: "prefix",
		html: `tag text prefix`,
	}}

	suffixComponents := []inlineEmojiSizeFilterTestCase{{
		name: "no suffix",
		html: ``,
	}, {
		name: "suffix",
		html: `tag text suffix`,
	}}

	// Now we can loop-create the test cases
	for _, emojiComponent := range emojiComponents {
		for _, heightComponent := range heightComponents {
			for _, closingTagComponent := range closingTagComponents {
				for _, prefixComponent := range prefixComponents {
					for _, suffixComponent := range suffixComponents {
						// We also need to create a loop to flip the order of the attributes
						for i := 0; i < 2; i++ {
							// Now we can create the test case. We'll vary the string casing in a moment before adding
							// the 3 resulting cases to the array.
							flipName := "height before emoji"
							firstAttr := heightComponent
							secondAttr := emojiComponent
							if i == 1 {
								flipName = "emoji before height"
								firstAttr = emojiComponent
								secondAttr = heightComponent
							}
							html := fmt.Sprintf(
								`%s<img src="mxc://example.org/abc" %s %s %s%s`,
								prefixComponent.html,
								firstAttr.html,
								secondAttr.html,
								closingTagComponent.html,
								suffixComponent.html,
							)
							lower, upper, mixed := variateHtml(html)
							isSpammy := emojiComponent.isSpammy || heightComponent.isSpammy // only these components affect spammy-ness
							baseName := strings.Join([]string{
								emojiComponent.name,
								heightComponent.name,
								closingTagComponent.name,
								prefixComponent.name,
								suffixComponent.name,
								flipName,
							}, "; ")

							inlineEmojiSizeFilterTestCases = append(
								inlineEmojiSizeFilterTestCases,
								inlineEmojiSizeFilterTestCase{
									name:     baseName + "; lowercase",
									html:     lower,
									isSpammy: isSpammy,
								}, inlineEmojiSizeFilterTestCase{
									name:     baseName + "; uppercase",
									html:     upper,
									isSpammy: isSpammy,
								}, inlineEmojiSizeFilterTestCase{
									name:     baseName + "; mixed",
									html:     mixed,
									isSpammy: isSpammy,
								},
							)

							// If there's no prefix or suffix then we add a test case for multiple tags in the HTML.
							// We'd normally just loop over the tests we did create and combine them, but as of writing
							// that would be 3168x3168 tests for a total of 10,036,224 total cases. That's a huge number
							// which Go struggles to actually run efficiently.
							//
							// We put a non-spammy tag first to try tripping up any tokenizer loops in the filter.
							if prefixComponent.html == "" && suffixComponent.html == "" {
								inlineEmojiSizeFilterTestCases = append(
									inlineEmojiSizeFilterTestCases,
									inlineEmojiSizeFilterTestCase{
										name:     baseName + "; multiple tags",
										html:     fmt.Sprintf(`Multiple tags: <img src="mxc://example.org/abc" data-mx-emoticon="" height="16" /> %s`, html),
										isSpammy: isSpammy,
									},
								)
							}
						}
					}
				}
			}
		}
	}
}

func TestInlineEmojiSizeFilter(t *testing.T) {
	cnf := &SetConfig{
		CommunityConfig: &config.CommunityConfig{
			InlineEmojiSizeFilterMaxHeightPixels: internal.Pointer(inlineEmojiSizeFilterMaxPixels),
		},
		Groups: []*SetGroupConfig{{
			EnabledNames:           []string{InlineEmojiSizeFilterName},
			MinimumSpamVectorValue: 0.0,
			MaximumSpamVectorValue: 1.0,
		}},
	}
	memStorage := test.NewMemoryStorage(t)
	defer memStorage.Close()
	ps := test.NewMemoryPubsub(t)
	defer ps.Close()
	set, err := NewSet(cnf, memStorage, ps, test.NewMatrixNotifier(t), nil)
	assert.NoError(t, err)
	assert.NotNil(t, set)

	for i, tc := range inlineEmojiSizeFilterTestCases {
		t.Run(tc.name, func(t *testing.T) {
			event := test.MustMakePDU(&test.BaseClientEvent{
				EventId: fmt.Sprintf("$testcase%d", i),
				RoomId:  "!foo:example.org",
				Type:    "m.room.message",
				Content: map[string]any{
					"msgtype":        "doesn't matter",
					"body":           "doesn't matter",
					"format":         "doesn't matter",
					"formatted_body": tc.html,
				},
			})
			t.Log("Testing event with HTML:", tc.html)

			vecs, err := set.CheckEvent(context.Background(), event, nil)
			assert.NoError(t, err)
			if tc.isSpammy {
				assert.Equal(t, 1.0, vecs.GetVector(classification.Spam))
			} else {
				// Because the filter doesn't flag things as "not spam", the seed value should survive
				assert.Equal(t, 0.5, vecs.GetVector(classification.Spam))
			}
		})
	}
}

func TestInlineEmojiSizeFilterRejectsInvalidHtml(t *testing.T) {
	cnf := &SetConfig{
		CommunityConfig: &config.CommunityConfig{
			InlineEmojiSizeFilterMaxHeightPixels: internal.Pointer(inlineEmojiSizeFilterMaxPixels),
		},
		Groups: []*SetGroupConfig{{
			EnabledNames:           []string{InlineEmojiSizeFilterName},
			MinimumSpamVectorValue: 0.0,
			MaximumSpamVectorValue: 1.0,
		}},
	}
	memStorage := test.NewMemoryStorage(t)
	defer memStorage.Close()
	ps := test.NewMemoryPubsub(t)
	defer ps.Close()
	set, err := NewSet(cnf, memStorage, ps, test.NewMatrixNotifier(t), nil)
	assert.NoError(t, err)
	assert.NotNil(t, set)

	// Extract the filter to configure the tokenizer into an error condition
	f, ok := set.groups[0].filters[0].(*InstancedInlineEmojiSizeFilter)
	assert.True(t, ok)
	f.tokenizerCallback = func(tokenizer *html.Tokenizer) {
		tokenizer.SetMaxBuf(1) // this will cause tokenizer.Next() to return html.ErrorToken
	}

	// Send an event through the filter which has more than 1 byte of HTML
	event := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$invalidhtml",
		RoomId:  "!foo:example.org",
		Type:    "m.room.message",
		Content: map[string]any{
			"msgtype":        "doesn't matter",
			"body":           "doesn't matter",
			"format":         "doesn't matter",
			"formatted_body": "<p>more than 1 byte</p>",
		},
	})
	_, err = set.CheckEvent(context.Background(), event, nil)
	assert.ErrorContains(t, err, html.ErrBufferExceeded.Error())
}

// variateHtml creates 3 versions of the input HTML (always in this order):
// 1. Lowercase
// 2. Uppercase
// 3. Mixed case. Tags will oscillate between lowercase and uppercase.
func variateHtml(html string) (string, string, string) {
	mixedCaseHtml := ""
	inTag := false
	clock := 0
	for _, c := range html {
		if c == '<' {
			inTag = true
		} else if c == '>' {
			inTag = false
		}
		if !inTag {
			mixedCaseHtml += string(c)
			continue
		}
		clock++
		if clock%2 == 0 {
			mixedCaseHtml += strings.ToLower(string(c))
		} else {
			mixedCaseHtml += strings.ToUpper(string(c))
		}
	}

	return strings.ToLower(html), strings.ToUpper(html), mixedCaseHtml
}

func TestVariateHtml(t *testing.T) {
	t.Parallel()

	lower, upper, mixed := variateHtml(`Non-tag text unaffected in mixed case. See <example tag="value">between tags is also unaffected</example>`)
	assert.Equal(t, `non-tag text unaffected in mixed case. see <example tag="value">between tags is also unaffected</example>`, lower)
	assert.Equal(t, `NON-TAG TEXT UNAFFECTED IN MIXED CASE. SEE <EXAMPLE TAG="VALUE">BETWEEN TAGS IS ALSO UNAFFECTED</EXAMPLE>`, upper)
	assert.Equal(t, `Non-tag text unaffected in mixed case. See <eXaMpLe tAg="VaLuE">between tags is also unaffected</ExAmPlE>`, mixed)
}
