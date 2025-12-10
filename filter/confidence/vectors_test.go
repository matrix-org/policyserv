package confidence

import (
	"fmt"
	"testing"

	"github.com/matrix-org/policyserv/filter/classification"
	"github.com/stretchr/testify/assert"
)

func TestGetSetVectors(t *testing.T) {
	v := NewConfidenceVectors()

	// Set some values to ensure the map can be used right away
	v.SetVector(classification.Spam, 1.0)
	v.SetVector(classification.Mentions, 0.8)
	v.SetVector(classification.Frequency, 0.2)
	v.SetVector(classification.NonCompliance, 0.0)
	v.SetVector(classification.Volumetric.Invert(), 1.0)

	// Make sure those values were stored
	assert.Equal(t, 1.0, v.GetVector(classification.Spam), "spam should be 1.0")
	assert.Equal(t, 0.8, v.GetVector(classification.Mentions), "mentions should be 0.8")
	assert.Equal(t, 0.2, v.GetVector(classification.Frequency), "frequency should be 0.2")
	assert.Equal(t, 0.0, v.GetVector(classification.NonCompliance), "non_compliance should be 0.0")
	assert.Equal(t, 0.0, v.GetVector(classification.Volumetric), "volumetric should be 0.0")
	assert.Equal(t, 1.0, v.GetVector(classification.Volumetric.Invert()), "inverted_volumetric should be 1.0")

	// Overwrite a value and ensure it gets read back fine
	v.SetVector(classification.Spam, 0.2)
	assert.Equal(t, 0.2, v.GetVector(classification.Spam), "overwritten spam should be 0.2")

	// Get a value that hasn't been set
	assert.Equal(t, 0.0, v.GetVector(classification.Volumetric), "unset value should be 0.0")
}

func TestSetVectorRangeTooHigh(t *testing.T) {
	vals := []float64{1.1, 2.0, 1.00000000001, -0.1, -1.0, -0.00000000001}
	for _, val := range vals {
		func(val float64) {
			v := NewConfidenceVectors()
			defer func() {
				if r := recover(); r != nil {
					if err, ok := r.(error); ok {
						assert.ErrorContains(t, err, "vector out of range", fmt.Sprintf("expected out of range on %f", val))
					} else {
						t.Errorf("Unexpected panic on %f: %#v %s", val, r, r)
					}
				} else {
					t.Errorf("Expected panic on %f", val)
				}
			}()
			v.SetVector(classification.Spam, val)
		}(val)
	}
}

func TestAverageVectors(t *testing.T) {
	v1 := NewConfidenceVectors()
	v1.SetVector(classification.Spam, 1.0)

	v2 := NewConfidenceVectors()
	v2.SetVector(classification.Spam, 1.0)
	v2.SetVector(classification.Mentions, 0.5)
	v2.SetVector(classification.Frequency.Invert(), 0.75)

	avg := AverageVectors(v1, v2)
	assert.Equal(t, 1.0, avg.GetVector(classification.Spam), "spam should be 1.0")
	assert.Equal(t, 0.25, avg.GetVector(classification.Mentions), "mentions should be 0.25")
	assert.Equal(t, 0.125, avg.GetVector(classification.Frequency), "frequency should be 0.125")
}
