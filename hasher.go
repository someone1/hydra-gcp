package hydragcp

import (
	"context"
	"fmt"

	"github.com/ory/fosite"
	"go.opencensus.io/trace"
)

// NewTracedHasher will wrap the given fosite.Hasher with one that will add spans to existing traces
func NewTracedHasher(hasher fosite.Hasher, attrs []trace.Attribute) fosite.Hasher {
	return &tracedHasher{
		hashOp:    fmt.Sprintf("hydra.%T.hash", hasher),
		compareOp: fmt.Sprintf("hydra.%T.compare", hasher),
		attrs:     attrs,
		Hasher:    hasher,
	}
}

type tracedHasher struct {
	hashOp    string
	compareOp string
	attrs     []trace.Attribute

	fosite.Hasher
}

func (t *tracedHasher) Hash(ctx context.Context, data []byte) ([]byte, error) {
	tctx, span := trace.StartSpan(ctx, t.hashOp)
	defer span.End()
	span.AddAttributes(t.attrs...)
	return t.Hasher.Hash(tctx, data)
}

func (t *tracedHasher) Compare(ctx context.Context, hash, data []byte) error {
	tctx, span := trace.StartSpan(ctx, t.compareOp)
	defer span.End()
	span.AddAttributes(t.attrs...)
	return t.Hasher.Compare(tctx, hash, data)
}
