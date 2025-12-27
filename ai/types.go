package ai

import (
	"context"

	"github.com/matrix-org/gomatrixserverlib"
	"github.com/matrix-org/policyserv/filter/classification"
	"github.com/matrix-org/policyserv/media"
)

type Input struct {
	Event  gomatrixserverlib.PDU
	Medias []*media.Item
}

type Provider[ConfigT any] interface {
	CheckEvent(ctx context.Context, cnf ConfigT, input *Input) ([]classification.Classification, error)
}
