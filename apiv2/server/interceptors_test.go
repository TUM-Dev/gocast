package apiv2

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"

	"go.uber.org/mock/gomock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
)

// Authentication moved into an interceptor, so what a handler sees is decided in two
// places. These pin the seam: the interceptor publishes, getCurrent reads, and a
// handler reached without an interceptor still works.

func TestGetCurrentReadsTheResolvedCaller(t *testing.T) {
	ctrl := gomock.NewController(t)

	// The DAO must not be touched: re-resolving is the work the interceptor removes.
	usersMock := mock_dao.NewMockUsersDao(ctrl)
	usersMock.EXPECT().GetUserByID(gomock.Any(), gomock.Any()).Times(0)

	api := &API{dao: dao.DaoWrapper{UsersDao: usersMock}, log: slog.Default()}

	resolved := &model.User{Model: gorm.Model{ID: 7}, Name: "Hansi"}
	ctx := context.WithValue(context.Background(), callerKey{}, &caller{user: resolved})

	got, err := api.getCurrent(ctx)
	if err != nil {
		t.Fatalf("getCurrent: %v", err)
	}
	if got != resolved {
		t.Errorf("got %v, want the user the interceptor resolved", got)
	}
}

func TestGetCurrentReportsWhyResolutionFailed(t *testing.T) {
	api := &API{log: slog.Default()}

	// A handler answering 401 rather than 403 must tell a rejected token from none.
	rejected := errors.New("token expired")
	ctx := context.WithValue(context.Background(), callerKey{}, &caller{err: rejected})

	user, err := api.getCurrent(ctx)
	if user != nil {
		t.Errorf("got user %v, want none", user)
	}
	if !errors.Is(err, rejected) {
		t.Errorf("got error %v, want %v", err, rejected)
	}
}

func TestGetCurrentFallsBackWithoutAnInterceptor(t *testing.T) {
	api := &API{log: slog.Default()}

	// What a handler called directly from a test looks like: it must resolve rather
	// than panic on the missing context value.
	if _, err := api.getCurrent(context.Background()); err == nil {
		t.Error("expected an anonymous context to produce an error")
	}
}

func TestResolveCallerPublishesTheCallerOnce(t *testing.T) {
	api := &API{log: slog.Default()}

	var seen *caller
	handler := func(ctx context.Context, _ any) (any, error) {
		c, ok := ctx.Value(callerKey{}).(*caller)
		if !ok {
			t.Fatal("handler was called without a resolved caller in its context")
		}
		seen = c

		// Every read inside one request must be the same resolution, not a new one.
		first, _ := api.getCurrent(ctx)
		second, _ := api.getCurrent(ctx)
		if first != second {
			t.Error("getCurrent resolved the caller more than once")
		}
		return "ok", nil
	}

	info := &grpc.UnaryServerInfo{FullMethod: "/protobuf.API/getUser"}
	if _, err := api.resolveCaller(context.Background(), nil, info, handler); err != nil {
		t.Fatalf("resolveCaller: %v", err)
	}

	// Anonymous, but published all the same, so the handler never falls back.
	if seen == nil || seen.user != nil || seen.err == nil {
		t.Errorf("got %+v, want an anonymous caller with the reason recorded", seen)
	}
}

func TestLogRequestPassesTheHandlerResultThrough(t *testing.T) {
	api := &API{log: slog.Default()}
	info := &grpc.UnaryServerInfo{FullMethod: "/protobuf.API/getUser"}

	t.Run("success", func(t *testing.T) {
		resp, err := api.logRequest(context.Background(), nil, info,
			func(context.Context, any) (any, error) { return "body", nil })
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp != "body" {
			t.Errorf("got %v, want the handler's response", resp)
		}
	})

	t.Run("failure", func(t *testing.T) {
		// Logging must not reshape the error; the gateway turns its code into HTTP.
		want := status.Error(codes.PermissionDenied, "nope")
		_, err := api.logRequest(context.Background(), nil, info,
			func(context.Context, any) (any, error) { return nil, want })
		if status.Code(err) != codes.PermissionDenied {
			t.Errorf("got code %v, want %v", status.Code(err), codes.PermissionDenied)
		}
	})
}

// An expired token used to be indistinguishable from none on anonymous endpoints,
// silently downgrading a signed-in client. These pin the two apart.
func TestCurrentOrAnonymous(t *testing.T) {
	api := &API{log: slog.Default()}

	withCaller := func(u *model.User, err error) context.Context {
		return context.WithValue(context.Background(), callerKey{}, &caller{user: u, err: err})
	}

	t.Run("no credentials is an anonymous caller, not an error", func(t *testing.T) {
		user, err := api.currentOrAnonymous(withCaller(nil, fmt.Errorf("%w: no bearer token or session cookie", ErrNoCredentials)))
		if err != nil {
			t.Fatalf("anonymous request errored: %v", err)
		}
		if user != nil {
			t.Errorf("got user %v, want none", user)
		}
	})

	t.Run("a rejected credential is a 401 so the client refreshes", func(t *testing.T) {
		_, err := api.currentOrAnonymous(withCaller(nil, errors.New("token is expired")))
		if got := status.Code(err); got != codes.Unauthenticated {
			t.Errorf("code = %v, want %v — a 403 would leave the client with no way to recover", got, codes.Unauthenticated)
		}
	})

	t.Run("a signed-in caller is returned", func(t *testing.T) {
		hansi := &model.User{Model: gorm.Model{ID: 7}}
		user, err := api.currentOrAnonymous(withCaller(hansi, nil))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user != hansi {
			t.Errorf("got %v, want the resolved user", user)
		}
	})
}

// The sentinel only helps if the paths meaning "nothing presented" carry it.
func TestResolveCurrentReportsMissingCredentials(t *testing.T) {
	api := &API{log: slog.Default()}

	tests := map[string]context.Context{
		"no metadata at all": context.Background(),
		"metadata without a credential": metadata.NewIncomingContext(
			context.Background(), metadata.MD{}),
		"a cookie header with no jwt cookie": metadata.NewIncomingContext(
			context.Background(), metadata.Pairs("grpcgateway-cookie", "theme=dark")),
	}

	for name, ctx := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := api.resolveCurrent(ctx)
			if !errors.Is(err, ErrNoCredentials) {
				t.Errorf("got %v, want it to wrap ErrNoCredentials", err)
			}
		})
	}
}
