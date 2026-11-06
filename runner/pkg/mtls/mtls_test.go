package mtls

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

type testCA struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
	dir  string
}

func newCA(t *testing.T) *testCA {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, _ := x509.ParseCertificate(der)
	ca := &testCA{cert: cert, key: key, dir: t.TempDir()}
	writePEM(t, filepath.Join(ca.dir, "ca.pem"), "CERTIFICATE", der)
	return ca
}

func writePEM(t *testing.T, path, typ string, der []byte) {
	t.Helper()
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: typ, Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
}

// issue signs a certificate for name, valid as both server and client.
func (ca *testCA) issue(t *testing.T, name string) Files {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: name},
		DNSNames:     []string{name},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca.cert, &key.PublicKey, ca.key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, _ := x509.MarshalECPrivateKey(key)
	f := Files{
		Cert: filepath.Join(ca.dir, name+".pem"),
		Key:  filepath.Join(ca.dir, name+".key"),
		CA:   filepath.Join(ca.dir, "ca.pem"),
	}
	writePEM(t, f.Cert, "CERTIFICATE", der)
	writePEM(t, f.Key, "EC PRIVATE KEY", keyDER)
	return f
}

// serve starts a health server that only accepts the peer allowed.
func serve(t *testing.T, server Files, allowed string) string {
	t.Helper()
	creds, err := server.ServerCredentials()
	if err != nil {
		t.Fatal(err)
	}
	s := grpc.NewServer(grpc.Creds(creds), grpc.ChainUnaryInterceptor(RequirePeer(allowed)))
	healthpb.RegisterHealthServer(s, health.NewServer())
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = s.Serve(lis) }()
	t.Cleanup(s.Stop)
	return lis.Addr().String()
}

func check(addr string, opt grpc.DialOption) error {
	conn, err := grpc.NewClient(addr, opt)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = healthpb.NewHealthClient(conn).Check(ctx, &healthpb.HealthCheckRequest{})
	return err
}

func TestHandshake(t *testing.T) {
	ca := newCA(t)
	gocast := ca.issue(t, "tumlive")
	runnerA := ca.issue(t, "runner-a.example")
	runnerB := ca.issue(t, "runner-b.example")
	addr := serve(t, runnerA, "tumlive")

	dial := func(f Files, serverName string) grpc.DialOption {
		creds, err := f.ClientCredentials(serverName)
		if err != nil {
			t.Fatal(err)
		}
		return grpc.WithTransportCredentials(creds)
	}

	t.Run("gocast reaches the runner", func(t *testing.T) {
		if err := check(addr, dial(gocast, "runner-a.example")); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("another runner is rejected", func(t *testing.T) {
		if got := status.Code(check(addr, dial(runnerB, "runner-a.example"))); got != codes.PermissionDenied {
			t.Fatalf("code = %v, want PermissionDenied", got)
		}
	})
	t.Run("a server with the wrong name is rejected", func(t *testing.T) {
		if err := check(addr, dial(gocast, "runner-b.example")); err == nil {
			t.Fatal("handshake with a mismatching server name succeeded")
		}
	})
	t.Run("plaintext is rejected", func(t *testing.T) {
		if err := check(addr, grpc.WithTransportCredentials(insecure.NewCredentials())); err == nil {
			t.Fatal("plaintext call succeeded")
		}
	})
	t.Run("a certificate from another CA is rejected", func(t *testing.T) {
		other := newCA(t).issue(t, "tumlive")
		other.CA = gocast.CA // trust our server, but present a foreign client cert
		if err := check(addr, dial(other, "runner-a.example")); err == nil {
			t.Fatal("foreign client certificate accepted")
		}
	})
}

func TestEnabled(t *testing.T) {
	for _, tc := range []struct {
		name    string
		f       Files
		on      bool
		wantErr bool
	}{
		{"none", Files{}, false, false},
		{"all", Files{Cert: "a", Key: "b", CA: "c"}, true, false},
		{"partial", Files{Cert: "a"}, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			on, err := tc.f.Enabled()
			if on != tc.on || (err != nil) != tc.wantErr {
				t.Fatalf("Enabled() = %v, %v", on, err)
			}
		})
	}
}

func TestCertReload(t *testing.T) {
	ca := newCA(t)
	server := ca.issue(t, "runner-a.example")
	addr := serve(t, server, "tumlive")
	gocast := ca.issue(t, "tumlive")
	creds, _ := gocast.ClientCredentials("runner-a.example")
	if err := check(addr, grpc.WithTransportCredentials(creds)); err != nil {
		t.Fatal(err)
	}
	// Replace the client certificate on disk with one of another name: the next handshake
	// must present it without the credentials being rebuilt.
	swapped := ca.issue(t, "intruder")
	b, _ := os.ReadFile(swapped.Cert)
	_ = os.WriteFile(gocast.Cert, b, 0o600)
	b, _ = os.ReadFile(swapped.Key)
	_ = os.WriteFile(gocast.Key, b, 0o600)
	if got := status.Code(check(addr, grpc.WithTransportCredentials(creds))); got != codes.PermissionDenied {
		t.Fatalf("code = %v, want PermissionDenied after swap", got)
	}
}
