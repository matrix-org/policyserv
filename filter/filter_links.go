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
//   - If an AllowList is specified, any URL NOT matching the allow list is flagged as spam.
//   - If a DenyList is specified, any URL matching the deny list is flagged as spam.
//   - If both are specified, a URL is allowed if it matches the AllowList AND does not match the DenyList.
//     In practice, the AllowList check happens first; if a URL passes, it is then checked against the DenyList.
type LinkFilter struct{}

func (l *LinkFilter) MakeFor(set *Set) (Instanced, error) {
	return &InstancedLinkFilter{
		set:       set,
		allowList: internal.Dereference(set.communityConfig.LinkFilterAllowList),
		denyList:  internal.Dereference(set.communityConfig.LinkFilterDenyList),
	}, nil
}

type InstancedLinkFilter struct {
	set       *Set
	allowList []string
	denyList  []string
}

func (f *InstancedLinkFilter) Name() string {
	return LinkFilterName
}

func (f *InstancedLinkFilter) CheckEvent(ctx context.Context, input *Input) ([]classification.Classification, error) {
	// If neither list is configured, this filter has no opinion.
	if len(f.allowList) == 0 && len(f.denyList) == 0 {
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
		if !f.isURLAllowed(url) {
			return []classification.Classification{classification.Spam}, nil
		}
	}

	return nil, nil
}

// isURLAllowed checks if a URL is allowed based on the configured allow and deny lists.
func (f *InstancedLinkFilter) isURLAllowed(url string) bool {
	// If an allow list is configured, the URL must match at least one pattern.
	if len(f.allowList) > 0 {
		matched := false
		for _, pattern := range f.allowList {
			if glob.Glob(pattern, url) {
				matched = true
				break
			}
		}
		if !matched {
			return false // URL does not match allow list.
		}
	}

	// If a deny list is configured, the URL must NOT match any pattern.
	if len(f.denyList) > 0 {
		for _, pattern := range f.denyList {
			if glob.Glob(pattern, url) {
				return false // URL matches deny list.
			}
		}
	}

	return true
}
