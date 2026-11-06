package runner_manager

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"math/big"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"github.com/tum-dev/gocast/runner/pkg/mtls"

	"github.com/TUM-Dev/gocast/model"
)

func TestRecordCert(t *testing.T) {
	notAfter := time.Date(2028, 1, 1, 0, 0, 0, 0, time.UTC)
	cert := &x509.Certificate{
		Raw:          []byte("der"),
		SerialNumber: big.NewInt(255),
		NotAfter:     notAfter,
		DNSNames:     []string{"a.example", "b.example"},
	}
	ctx := peer.NewContext(context.Background(), &peer.Peer{
		AuthInfo: credentials.TLSInfo{State: tls.ConnectionState{PeerCertificates: []*x509.Certificate{cert}}},
	})

	var r model.Runner
	recordCert(ctx, &r)

	if r.CertSerial != "ff" || r.CertNames != "a.example,b.example" || r.CertFingerprint == "" {
		t.Fatalf("unexpected record: %+v", r)
	}
	if r.CertNotAfter == nil || !r.CertNotAfter.Equal(notAfter) {
		t.Fatalf("CertNotAfter = %v, want %v", r.CertNotAfter, notAfter)
	}
}

// Without mutual TLS there is no certificate, and what a runner had must survive a heartbeat.
func TestRecordCertKeepsRecordWithoutTLS(t *testing.T) {
	r := model.Runner{CertSerial: "ff"}
	recordCert(context.Background(), &r)
	if r.CertSerial != "ff" {
		t.Fatalf("CertSerial = %q, want it kept", r.CertSerial)
	}
}

func TestRequirePeer(t *testing.T) {
	ctx := peer.NewContext(context.Background(), &peer.Peer{
		AuthInfo: credentials.TLSInfo{State: tls.ConnectionState{PeerCertificates: []*x509.Certificate{
			{DNSNames: []string{"runner-a.example"}},
		}}},
	})
	on := &Manager{tls: mtls.Files{Cert: "c", Key: "k", CA: "a"}}

	if err := on.requirePeer(ctx, "runner-a.example"); err != nil {
		t.Fatalf("own name rejected: %v", err)
	}
	if got := status.Code(on.requirePeer(ctx, "runner-b.example")); got != codes.PermissionDenied {
		t.Fatalf("another runner's name: code = %v, want PermissionDenied", got)
	}
	if err := (&Manager{}).requirePeer(ctx, "runner-b.example"); err != nil {
		t.Fatalf("mTLS off must allow all: %v", err)
	}
}
