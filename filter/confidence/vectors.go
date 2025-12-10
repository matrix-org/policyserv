package confidence

import (
	"fmt"

	"github.com/matrix-org/policyserv/filter/classification"
)

type Vectors map[classification.Classification]float64

func NewConfidenceVectors() Vectors {
	return make(map[classification.Classification]float64)
}

func (v Vectors) SetVector(classification classification.Classification, vector float64) {
	if vector > 1.0 || vector < 0.0 {
		// "Should never happen" - programmer error
		panic(fmt.Errorf("vector out of range: %f must be between 0.0 and 1.0", vector))
	}
	// We store non-inverted values.
	if classification.IsInverted() {
		vector = 1.0 - vector
		classification = classification.Invert()
	}
	v[classification] = vector
}

func (v Vectors) GetVector(classification classification.Classification) float64 {
	// We store non-inverted values.
	if classification.IsInverted() {
		classification = classification.Invert()
		return 1.0 - v[classification]
	}
	return v[classification]
}

func AverageVectors(vectors ...Vectors) Vectors {
	// Create a container
	combined := NewConfidenceVectors()

	// Add up all the vectors
	for _, other := range vectors {
		for cls, vec := range other {
			// Note: we don't need to check for inverted values here, because we store non-inverted values.
			combined[cls] += vec
		}
	}

	// Average them
	for cls, _ := range combined {
		combined[cls] /= float64(len(vectors))
	}

	// Return the result
	return combined
}
