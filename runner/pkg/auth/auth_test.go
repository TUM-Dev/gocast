package auth

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const secret = "s3cret"

func call(t *testing.T, ic grpc.UnaryServerInterceptor, md metadata.MD) (context.Context, error) {
	t.Helper()
	ctx := context.Background()
	if md != nil {
		ctx = metadata.NewIncomingContext(ctx, md)
	}
	var seen context.Context
	_, err := ic(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/t"}, func(ctx context.Context, _ any) (any, error) {
		seen = ctx
		return nil, nil
	})
	return seen, err
}

func TestServerInterceptorRejectsMissingOrWrongToken(t *testing.T) {
	ic := UnaryServerInterceptor(secret, false)
	cases := map[string]metadata.MD{
		"no metadata":     nil,
		"no header":       metadata.Pairs(),
		"wrong token":     metadata.Pairs("authorization", "Bearer nope"),
		"no bearer":       metadata.Pairs("authorization", secret),
		"prefix only":     metadata.Pairs("authorization", "Bearer "+secret[:3]),
		"two headers":     metadata.Pairs("authorization", "Bearer "+secret, "authorization", "Bearer "+secret),
		"empty authz":     metadata.Pairs("authorization", ""),
		"token too short": metadata.Pairs("authorization", "Bear"),
	}
	for name, md := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := call(t, ic, md)
			if status.Code(err) != codes.Unauthenticated {
				t.Fatalf("want Unauthenticated, got %v", err)
			}
		})
	}
}

func TestServerInterceptorAcceptsToken(t *testing.T) {
	ic := UnaryServerInterceptor(secret, false)
	for _, v := range []string{"Bearer " + secret, "bearer " + secret} {
		if _, err := call(t, ic, metadata.Pairs("authorization", v)); err != nil {
			t.Fatalf("%q: want success, got %v", v, err)
		}
	}
}

// An empty configured token must not mean "no auth".
func TestServerInterceptorEmptyTokenRejectsEverything(t *testing.T) {
	ic := UnaryServerInterceptor("", false)
	for _, v := range []string{"Bearer ", "Bearer x"} {
		if _, err := call(t, ic, metadata.Pairs("authorization", v)); status.Code(err) != codes.Unauthenticated {
			t.Fatalf("%q: want Unauthenticated, got %v", v, err)
		}
	}
}

func TestServerInterceptorHostname(t *testing.T) {
	ic := UnaryServerInterceptor(secret, true)
	_, err := call(t, ic, metadata.Pairs("authorization", "Bearer "+secret))
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("missing hostname: want Unauthenticated, got %v", err)
	}
	ctx, err := call(t, ic, metadata.Pairs("authorization", "Bearer "+secret, "x-gocast-runner", "runner1"))
	if err != nil {
		t.Fatalf("want success, got %v", err)
	}
	if h, ok := CallerHostname(ctx); !ok || h != "runner1" {
		t.Fatalf("CallerHostname = %q, %v", h, ok)
	}
	if _, ok := CallerHostname(context.Background()); ok {
		t.Fatal("CallerHostname on a plain context must report absent")
	}
}

// The client interceptor must produce exactly what the server interceptor accepts.
func TestClientAndServerInterceptorsAgree(t *testing.T) {
	client := UnaryClientInterceptor(secret, "runner1")
	server := UnaryServerInterceptor(secret, true)
	var outgoing context.Context
	invoker := func(ctx context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
		outgoing = ctx
		return nil
	}
	if err := client(context.Background(), "/t", nil, nil, nil, invoker); err != nil {
		t.Fatal(err)
	}
	md, _ := metadata.FromOutgoingContext(outgoing)
	ctx, err := call(t, server, md)
	if err != nil {
		t.Fatalf("server rejected what the client sent: %v", err)
	}
	if h, _ := CallerHostname(ctx); h != "runner1" {
		t.Fatalf("hostname = %q", h)
	}
}

func TestRecoveryInterceptor(t *testing.T) {
	ic := RecoveryInterceptor(slog.New(slog.NewTextHandler(io.Discard, nil)))
	_, err := ic(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/t"}, func(context.Context, any) (any, error) {
		panic("boom")
	})
	if status.Code(err) != codes.Internal {
		t.Fatalf("want Internal, got %v", err)
	}
	want := errors.New("handler error")
	_, err = ic(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/t"}, func(context.Context, any) (any, error) {
		return nil, want
	})
	if !errors.Is(err, want) {
		t.Fatalf("non-panicking errors must pass through, got %v", err)
	}
}
