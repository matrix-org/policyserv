package test

import (
	"context"
	"testing"

	"github.com/matrix-org/policyserv/content"
	"github.com/matrix-org/policyserv/filter/classification"
	"github.com/stretchr/testify/assert"
)

type scanExpectation struct {
	contentType content.Type
	content     []byte
	expected    []classification.Classification
	err         error
}

type MemoryContentScanner struct {
	T            *testing.T
	expectations []scanExpectation
}

func NewMemoryContentScanner(t *testing.T) *MemoryContentScanner {
	return &MemoryContentScanner{T: t}
}

func (m *MemoryContentScanner) Expect(contentType content.Type, content []byte, retClassifications []classification.Classification, retErr error) {
	m.expectations = append(m.expectations, scanExpectation{contentType, content, retClassifications, retErr})
}

func (m *MemoryContentScanner) Scan(ctx context.Context, contentType content.Type, content []byte) ([]classification.Classification, error) {
	assert.NotNil(m.T, ctx, "context is required")

	for _, exp := range m.expectations {
		if exp.contentType == contentType && string(exp.content) == string(content) {
			return exp.expected, exp.err
		}
	}

	assert.Fail(m.T, "unexpected scan")
	return nil, nil
}
