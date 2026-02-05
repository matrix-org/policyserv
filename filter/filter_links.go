package filter

import (
	"context"
	"regexp"

	"github.com/matrix-org/policyserv/filter/classification"
	"github.com/matrix-org/policyserv/internal"
	"github.com/ryanuber/go-glob"
)

const LinkFilterName = "LinkFilter"

// urlRegex is a regex to detect URLs in text. It looks for http:// or https:// followed by non-whitespace characters.
var urlRegex = regexp.MustCompile(`https?://[^\s"<>\x60]+`)

func init() {
	mustRegister(LinkFilterName, &LinkFilter{})
}

// LinkFilter is a filter that checks for URLs in the event content.
// It can be configured with an allow list and/or a deny list.
//   - If a DenyList is specified, any URL matching the deny list is flagged as spam (deny wins).
//   - If an AllowList is specified, any URL NOT matching the allow list is flagged as spam.
//   - Deny list takes priority over allow list.
type LinkFilter struct{}

func (l *LinkFilter) MakeFor(set *Set) (Instanced, error) {
	return &InstancedLinkFilter{
		set:             set,
		allowedUrlGlobs: internal.Dereference(set.communityConfig.LinkFilterAllowedUrlGlobs),
		deniedUrlGlobs:  internal.Dereference(set.communityConfig.LinkFilterDeniedUrlGlobs),
	}, nil
}

type InstancedLinkFilter struct {
	set             *Set
	allowedUrlGlobs []string
	deniedUrlGlobs  []string
}

func (f *InstancedLinkFilter) Name() string {
	return LinkFilterName
}

func (f *InstancedLinkFilter) CheckEvent(ctx context.Context, input *Input) ([]classification.Classification, error) {
	// If neither list is configured, this filter has no opinion.
	if len(f.allowedUrlGlobs) == 0 && len(f.deniedUrlGlobs) == 0 {
		return nil, nil
	}

	// Scan event content for URLs.
	content := string(input.Event.Content())
	urls := urlRegex.FindAllString(content, -1)

	// No URLs found, so nothing to check.
	if len(urls) == 0 {
		return nil, nil
	}

	for _, url := range urls {
		if !f.isUrlAllowed(url) {
			return []classification.Classification{classification.Spam}, nil
		}
	}

	return nil, nil
}

// isUrlAllowed checks if a URL is allowed based on the configured allow and deny lists.
// Deny list takes priority (deny wins).
func (f *InstancedLinkFilter) isUrlAllowed(url string) bool {
	// Check deny list first - deny wins.
	for _, pattern := range f.deniedUrlGlobs {
		if glob.Glob(pattern, url) {
			return false // URL matches deny list.
		}
	}

	// If an allow list is configured, the URL must match at least one pattern.
	if len(f.allowedUrlGlobs) > 0 {
		for _, pattern := range f.allowedUrlGlobs {
			if glob.Glob(pattern, url) {
				return true // URL matches allow list.
			}
		}
		return false // URL does not match allow list.
	}

	return true
}
