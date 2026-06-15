package ai

import (
	"context"

	"github.com/matrix-org/gomatrixserverlib"
	"github.com/matrix-org/policyserv/harms"
	"github.com/matrix-org/policyserv/media"
)

type Input struct {
	Event  gomatrixserverlib.PDU
	Medias []*media.Item
}

type Provider[ConfigT any] interface {
	CheckEvent(ctx context.Context, cnf ConfigT, input *Input) (*harms.ContentInfo, error)
}
