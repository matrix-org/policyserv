package filter

import (
	"context"
	"slices"
	"testing"

	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/filter/classification"
	"github.com/matrix-org/policyserv/filter/confidence"
	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
)

func TestSetGroupCheck(t *testing.T) {
	event := test.MustMakePDU(&test.BaseClientEvent{
		RoomId:  "!foo:example.org",
		EventId: "$test",
		Type:    "m.room.message",
		Content: make(map[string]any),
	})
	auditCtx, err := newAuditContext(&config.InstanceConfig{}, event, "")
	assert.NoError(t, err)
	assert.NotNil(t, auditCtx)
	input := &Input{
		Event:                        event,
		IncrementalConfidenceVectors: confidence.Vectors{classification.Spam: 0.5},
		auditContext:                 auditCtx,
	}
	sg := &setGroup{
		filters: []Instanced{&FixedInstancedFilter{
			T:             t,
			Expect:        input,
			ReturnClasses: []classification.Classification{classification.Spam, classification.Volumetric},
			ReturnErr:     nil,
		}},
		minSpamVectorValue: 0.3,
		maxSpamVectorValue: 0.8,
	}

	// Does it no-op when the spam vector is out of range?
	input.IncrementalConfidenceVectors.SetVector(classification.Spam, 0.1)
	vecs, err := sg.checkEvent(context.Background(), input)
	assert.NoError(t, err)
	assert.Equal(t, confidence.NewConfidenceVectors(), vecs) // no-op is to return a zero value
	input.IncrementalConfidenceVectors.SetVector(classification.Spam, 0.9)
	vecs, err = sg.checkEvent(context.Background(), input)
	assert.NoError(t, err)
	assert.Equal(t, confidence.NewConfidenceVectors(), vecs) // no-op is to return a zero value

	// Does it process the event through the filter?
	input.IncrementalConfidenceVectors.SetVector(classification.Spam, 0.5)
	vecs, err = sg.checkEvent(context.Background(), input)
	assert.NoError(t, err)
	classes := make([]classification.Classification, 0)
	for cls, _ := range vecs {
		classes = append(classes, cls)
	}
	expectClasses := sg.filters[0].(*FixedInstancedFilter).ReturnClasses
	// Note: we sort because the order of values is not guaranteed on our custom types.
	slices.Sort(classes)
	slices.Sort(expectClasses)
	assert.Equal(t, expectClasses, classes)
}
