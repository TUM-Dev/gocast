package errors

import (
	"errors"
	"fmt"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

// The 404-or-500 mapping was open-coded at five call sites; now there is one.
func TestFromGorm(t *testing.T) {
	t.Run("a missing row becomes a 404 with the caller's message", func(t *testing.T) {
		err := FromGorm(gorm.ErrRecordNotFound, "can't find course")

		if got := status.Code(err); got != codes.NotFound {
			t.Errorf("code = %v, want %v", got, codes.NotFound)
		}
		if msg := status.Convert(err).Message(); msg != "can't find course" {
			t.Errorf("message = %q, want the caller's", msg)
		}
	})

	t.Run("a wrapped missing row is still a 404", func(t *testing.T) {
		// DAOs wrap, so matching on equality would make every not-found a 500.
		wrapped := fmt.Errorf("loading course: %w", gorm.ErrRecordNotFound)

		if got := status.Code(FromGorm(wrapped, "nope")); got != codes.NotFound {
			t.Errorf("code = %v, want %v", got, codes.NotFound)
		}
	})

	t.Run("any other failure is a 500 and keeps its own message", func(t *testing.T) {
		err := FromGorm(errors.New("connection refused"), "can't find course")

		if got := status.Code(err); got != codes.Unknown {
			t.Errorf("code = %v, want %v", got, codes.Unknown)
		}
		// The not-found message must not be attached to a different failure: it
		// would send the client hunting for a typo while the database is down.
		if msg := status.Convert(err).Message(); msg != "connection refused" {
			t.Errorf("message = %q, want the underlying error", msg)
		}
	})
}
