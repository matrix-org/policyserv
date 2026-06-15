package test

import (
	"testing"

	"github.com/matrix-org/policyserv/notifiers"
	"github.com/matrix-org/policyserv/storage"
	"github.com/stretchr/testify/assert"
)

type MatrixNotifier struct {
	notifiers.MatrixNotifier

	t *testing.T
}

func NewMatrixNotifier(t *testing.T) *MatrixNotifier {
	return &MatrixNotifier{
		t: t,
	}
}

func (n *MatrixNotifier) Send(communityId string, plainText string, htmlText string) (string, error) {
	assert.NotEmpty(n.t, communityId, "communityId is required")
	assert.NotEmpty(n.t, plainText, "plainText is required")
	assert.NotEmpty(n.t, htmlText, "htmlText is required")
	return storage.NextId(), nil
}
