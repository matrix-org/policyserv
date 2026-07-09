package content

import (
	"context"

	"github.com/matrix-org/policyserv/harms"
)

type Scanner interface {
	Scan(ctx context.Context, contentType Type, content []byte) (*harms.ContentInfo, error)
}
