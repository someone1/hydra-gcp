package dscon

import (
	"cloud.google.com/go/datastore"
	"github.com/ory/x/sqlcon"
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func HandleError(err error) error {
	if got, want := status.Code(err), codes.AlreadyExists; got == want {
		return errors.Wrap(sqlcon.ErrUniqueViolation, got.String())
	}

	if got, want := status.Code(err), codes.NotFound; got == want {
		return errors.WithStack(sqlcon.ErrNoRows)
	}

	if err == datastore.ErrNoSuchEntity {
		return errors.WithStack(sqlcon.ErrNoRows)
	}

	return errors.WithStack(err)
}
