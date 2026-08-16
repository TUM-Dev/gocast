package apiv2

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"go.uber.org/mock/gomock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
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
