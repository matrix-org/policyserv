package classification

import "strings"

type Classification string

// Spam - When closer to 1, the event should be considered spam. When closer to 0, the event is neutral
// and should still be considered as "potentially spammy, though unlikely".
const Spam Classification = "spam"

// CSAM - When closer to 1, the event likely (or definitely) contains CSAM.
const CSAM Classification = "csam"

// Volumetric - When closer to 1, the event attempts to disrupt the timeline of the room.
const Volumetric Classification = "volumetric"

// Frequency - When closer to 1, the event (or similar) may have already been repeated a lot.
const Frequency Classification = "frequency"

// Mentions - When closer to 1, the event's mechanism for spam includes mentioning other users.
const Mentions Classification = "mentions"

// DAGAbuse - When closer to 1, the event appears to be attempting to disrupt the core structure of the room.
const DAGAbuse Classification = "dag_abuse"

// NonCompliance - When closer to 1, the event does not comply with the specification. Events classified as such
// should have already been rejected by "real" homeservers.
const NonCompliance Classification = "non_compliance"

// Unsafe - When closer to 1, the event was not able to be fully checked by the filter engine and therefore may be spammy.
const Unsafe Classification = "unsafe"

func (c Classification) String() string {
	if c.IsInverted() {
		return strings.TrimPrefix(string(c), "inverted_")
	}
	return string(c)
}

func (c Classification) IsInverted() bool {
	return strings.HasPrefix(string(c), "inverted_")
}

func (c Classification) Invert() Classification {
	if c.IsInverted() {
		return Classification(c.String())
	}
	return Classification("inverted_" + c.String())
}
