package content

import (
	"context"

	"github.com/matrix-org/policyserv/filter/classification"
)

type Scanner interface {
	Scan(ctx context.Context, contentType Type, content []byte) ([]classification.Classification, error)
}
