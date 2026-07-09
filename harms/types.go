package harms

import "slices"

// Harm - A harm identifier.
type Harm string

// ContentClass - A broad classification of content, aside from any specific harms. Typically, if
// content is ContentClassProhibited then it will be accompanied by one or more harms too. Defined
// content classes are ordered such that ContentClassNeutral < ContentClassAllowed < ContentClassProhibited.
type ContentClass int

const (
	// ContentClassNeutral - The default state for any content.
	ContentClassNeutral ContentClass = iota
	// ContentClassAllowed - The content is explicitly allowed.
	ContentClassAllowed
	// ContentClassProhibited - The content is explicitly prohibited.
	ContentClassProhibited
)

// String - override to make Sprintf work a bit better.
func (c ContentClass) String() string {
	return [...]string{"Neutral", "Allowed", "Prohibited"}[c]
}

// ContentInfo - Carries harm and class information for a given piece of content.
type ContentInfo struct {
	class ContentClass
	harms []Harm
}

// NewContentInfo - Creates a new ContentInfo instance with the given class and harms. If the class is
// ContentClassProhibited and no harms are specified, OtherGeneral is used. If any harms are specified,
// the class is automatically set to ContentClassProhibited.
func NewContentInfo(class ContentClass, harms ...Harm) *ContentInfo {
	if len(harms) == 0 && class == ContentClassProhibited {
		harms = []Harm{OtherGeneral}
	}
	if len(harms) > 0 {
		class = ContentClassProhibited
	}
	return &ContentInfo{
		class: class,
		harms: deduplicateHarms(harms),
	}
}

func (i *ContentInfo) Class() ContentClass {
	return i.class
}

func (i *ContentInfo) Harms() []Harm {
	// Some code will call Harms() and immediately expect that it will have zero or more values, which isn't always
	// the case. To add some safety, we force-set harms to an empty slice if it's nil.
	if i.harms == nil {
		i.harms = make([]Harm, 0)
	}
	return i.harms
}

// ProhibitedContent - Creates a ContentInfo instance with the specified harms and ContentClassProhibited.
// If no harms are specified, OtherGeneral is used.
func ProhibitedContent(harms ...Harm) *ContentInfo {
	if len(harms) == 0 {
		harms = []Harm{OtherGeneral}
	}
	return &ContentInfo{
		class: ContentClassProhibited,
		harms: deduplicateHarms(harms),
	}
}

// NeutralContent - Creates a ContentInfo instance with ContentClassNeutral.
func NeutralContent() *ContentInfo {
	return &ContentInfo{
		class: ContentClassNeutral,
	}
}

// AllowedContent - Creates a ContentInfo instance with ContentClassAllowed.
func AllowedContent() *ContentInfo {
	return &ContentInfo{
		class: ContentClassAllowed,
	}
}

func deduplicateHarms(harms []Harm) []Harm {
	// We sort primarily for tests (they compare ContentInfo instances), but also to make Compact() actually work.
	// Compact() removes duplicates when they're right next to each other, and Sort() does that for us.
	slices.Sort(harms)
	return slices.Compact(harms)
}
