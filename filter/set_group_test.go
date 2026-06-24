package filter

import (
	"context"
	"testing"

	"github.com/matrix-org/policyserv/harms"
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
	auditCtx, err := newAuditContext(test.NewMatrixNotifier(t), "default", event)
	assert.NoError(t, err)
	assert.NotNil(t, auditCtx)
	input := &EventInput{
		Event:        event,
		auditContext: auditCtx,
	}
	sg := &setGroup{
		filters: []Instanced{&FixedInstancedFilter{
			T:          t,
			Expect:     input,
			ExpectText: "hello world",
			ReturnInfo: harms.ProhibitedContent(harms.SpamGeneral, harms.SpamFlooding),
			ReturnErr:  nil,
		}},
		runOnClasses: []harms.ContentClass{harms.ContentClassAllowed}, // we only want to capture a specific class of events
	}

	// Note: we test text and events at the same time

	// Does it no-op when the content class is wrong?
	info, err := sg.checkEvent(context.Background(), harms.NeutralContent(), input) // runOnClasses uses ContentClassAllowed
	assert.NoError(t, err)
	test.AssertEqualContentInfo(t, harms.NeutralContent(), info) // no-op is neutral
	info, err = sg.checkText(context.Background(), harms.NeutralContent(), "hello world")
	assert.NoError(t, err)
	test.AssertEqualContentInfo(t, harms.NeutralContent(), info) // no-op is neutral

	info, err = sg.checkEvent(context.Background(), harms.ProhibitedContent(harms.SpamFraud), input)
	assert.NoError(t, err)
	test.AssertEqualContentInfo(t, harms.NeutralContent(), info) // no-op is neutral
	info, err = sg.checkText(context.Background(), harms.ProhibitedContent(harms.SpamFraud), "hello world")
	assert.NoError(t, err)
	test.AssertEqualContentInfo(t, harms.NeutralContent(), info) // no-op is neutral

	// Does it process the event through the filter?
	info, err = sg.checkEvent(context.Background(), harms.AllowedContent(), input) // runOnClasses uses ContentClassAllowed
	assert.NoError(t, err)
	test.AssertEqualContentInfo(t, harms.ProhibitedContent(harms.SpamGeneral, harms.SpamFlooding), info)
	info, err = sg.checkText(context.Background(), harms.AllowedContent(), "hello world") // runOnClasses uses ContentClassAllowed
	assert.NoError(t, err)
	test.AssertEqualContentInfo(t, harms.ProhibitedContent(harms.SpamGeneral, harms.SpamFlooding), info)
}
