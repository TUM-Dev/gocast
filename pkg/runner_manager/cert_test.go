package runner_manager

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"math/big"
	"testing"
	"time"

	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"

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
