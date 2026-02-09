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
// It's configured with a deny list and an allow list. Matches against the deny list win.
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

func (f *InstancedLinkFilter) isUrlAllowed(url string) bool {
	for _, pattern := range f.deniedUrlGlobs {
		if glob.Glob(pattern, url) {
			return false
		}
	}

	if len(f.allowedUrlGlobs) > 0 {
		for _, pattern := range f.allowedUrlGlobs {
			if glob.Glob(pattern, url) {
				return true
			}
		}
		return false
	}

	return true
}
