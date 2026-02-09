package filter

import (
	"context"
	"errors"
	"testing"

	"github.com/matrix-org/policyserv/filter/classification"
	"github.com/stretchr/testify/assert"
)

const FixedFilterName = "FixedFilter"
const ErrorFilterName = "ErrorFilter"

func init() {
	mustRegister(FixedFilterName, &FixedCanBeInstancedFilter{})
	mustRegister(ErrorFilterName, &ErrorFilter{})
}

// FixedCanBeInstancedFilter - A CanBeInstanced filter which exists for testing purposes
type FixedCanBeInstancedFilter struct {
}

func (f *FixedCanBeInstancedFilter) MakeFor(set *Set) (Instanced, error) {
	return &FixedInstancedFilter{
		Set: set,
		// other fields filled by tests
	}, nil
}

// FixedInstancedFilter - An Instanced filter which exists for testing purposes
type FixedInstancedFilter struct {
	T             *testing.T
	Set           *Set
	Expect        *EventInput
	ReturnClasses []classification.Classification
	ReturnErr     error
}

func (f *FixedInstancedFilter) Name() string {
	return FixedFilterName
}

func (f *FixedInstancedFilter) CheckEvent(ctx context.Context, input *EventInput) ([]classification.Classification, error) {
	assert.NotNil(f.T, ctx, "context is required")

	if f.Expect != nil {
		// Test that the audit context is set, either to a value defined by the test or to a non-nil value
		if f.Expect.auditContext != nil {
			assert.Equal(f.T, f.Expect.auditContext, input.auditContext)
		} else {
			assert.NotNil(f.T, input.auditContext)
		}

		// Check for similarity rather than precise equality to avoid complaining about auditContext being different
		assert.EqualExportedValues(f.T, f.Expect, input)
	} else {
		assert.Equal(f.T, f.Expect, input)
	}

	return f.ReturnClasses, f.ReturnErr
}

type ErrorFilter struct {
}

func (f *ErrorFilter) MakeFor(set *Set) (Instanced, error) {
	return nil, errors.New("this will always fail")
}
