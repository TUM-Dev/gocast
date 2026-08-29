package apiv2

import (
	"testing"

	"google.golang.org/grpc/metadata"
)

// The v2 API accepts a bearer token or the session cookie the old pages rely on.
// These pin which wins and what counts as present.

func TestExtractJWTFromMetadata(t *testing.T) {
	var api API

	tests := []struct {
		name string
		md   metadata.MD
		want string
		// wantErr means no usable credential was found at all.
		wantErr bool
	}{
		{
			name: "bearer token",
			md:   metadata.Pairs("authorization", "Bearer token-from-header"),
			want: "token-from-header",
		},
		{
			name: "session cookie",
			md:   metadata.Pairs("grpcgateway-cookie", "jwt=token-from-cookie"),
			want: "token-from-cookie",
		},
		{
			name: "bearer wins over cookie",
			md: metadata.MD{
				"authorization":      []string{"Bearer token-from-header"},
				"grpcgateway-cookie": []string{"jwt=token-from-cookie"},
			},
			want: "token-from-header",
		},
		{
			name: "cookie among others",
			md:   metadata.Pairs("grpcgateway-cookie", "theme=dark; jwt=token-from-cookie; other=1"),
			want: "token-from-cookie",
		},
		{
			name: "empty bearer falls through to cookie",
			md: metadata.MD{
				"authorization":      []string{"Bearer   "},
				"grpcgateway-cookie": []string{"jwt=token-from-cookie"},
			},
			want: "token-from-cookie",
		},
		{
			name:    "non-bearer authorization scheme is ignored",
			md:      metadata.Pairs("authorization", "Basic dXNlcjpwYXNz"),
			wantErr: true,
		},
		{
			name:    "no credentials",
			md:      metadata.MD{},
			wantErr: true,
		},
		{
			name:    "cookie header without a jwt cookie",
			md:      metadata.Pairs("grpcgateway-cookie", "theme=dark"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := api.extractJWTFromMetadata(tt.md)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected no credential to be found, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got token %q, want %q", got, tt.want)
			}
		})
	}
}
