// Package mtls builds the gRPC credentials for the link between gocast and the runners.
//
// Both directions use mutual TLS: the runner dials gocast to register and report, gocast dials
// the runner to hand out jobs. A peer is identified by the DNS names in its certificate, never
// by its address: on the overlay network the address and the container name change all the time.
package mtls

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"slices"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// Files are the paths of a PEM certificate, its key and the CA that signs the peers.
type Files struct {
	Cert string `yaml:"cert"`
	Key  string `yaml:"key"`
	CA   string `yaml:"ca"`
}

// Enabled tells whether mutual TLS is configured. Nothing set means off, which is how local
// development runs; a partial setting is an error, so that a typo in production can't silently
// leave the link in plaintext.
func (f Files) Enabled() (bool, error) {
	set := 0
	for _, p := range []string{f.Cert, f.Key, f.CA} {
		if p != "" {
			set++
		}
	}
	switch set {
	case 0:
		return false, nil
	case 3:
		return true, nil
	}
	return false, errors.New("mtls: cert, key and ca must be set together")
}

// Validate fails when mutual TLS is half configured or its files can't be used. A server calls
// it at startup, so that a broken production setup stops it and doesn't surface at the first
// handshake.
func (f Files) Validate() error {
	on, err := f.Enabled()
	if err != nil || !on {
		return err
	}
	_, err = f.ServerCredentials()
	return err
}

func (f Files) caPool() (*x509.CertPool, error) {
	pem, err := os.ReadFile(f.CA)
	if err != nil {
		return nil, fmt.Errorf("read ca: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return nil, errors.New("ca file holds no certificate")
	}
	return pool, nil
}

// keyPair is read on every handshake, so a certificate that is replaced on disk takes effect
// without a restart. A runner restart would wait for its running jobs.
func (f Files) keyPair() (*tls.Certificate, error) {
	c, err := tls.LoadX509KeyPair(f.Cert, f.Key)
	if err != nil {
		return nil, fmt.Errorf("load key pair: %w", err)
	}
	return &c, nil
}

// ServerCredentials makes the server require and verify a client certificate.
func (f Files) ServerCredentials() (credentials.TransportCredentials, error) {
	if _, err := f.keyPair(); err != nil {
		return nil, err
	}
	if _, err := f.caPool(); err != nil {
		return nil, err
	}
	return credentials.NewTLS(&tls.Config{
		MinVersion: tls.VersionTLS13,
		ClientAuth: tls.RequireAndVerifyClientCert,
		GetConfigForClient: func(*tls.ClientHelloInfo) (*tls.Config, error) {
			cert, err := f.keyPair()
			if err != nil {
				return nil, err
			}
			pool, err := f.caPool()
			if err != nil {
				return nil, err
			}
			return &tls.Config{
				MinVersion:   tls.VersionTLS13,
				ClientAuth:   tls.RequireAndVerifyClientCert,
				Certificates: []tls.Certificate{*cert},
				ClientCAs:    pool,
			}, nil
		},
	}), nil
}

// ClientCredentials verifies the server's certificate against serverName, which is the name
// the peer is registered under and not necessarily the name that is dialed.
func (f Files) ClientCredentials(serverName string) (credentials.TransportCredentials, error) {
	pool, err := f.caPool()
	if err != nil {
		return nil, err
	}
	if _, err := f.keyPair(); err != nil {
		return nil, err
	}
	return credentials.NewTLS(&tls.Config{
		MinVersion: tls.VersionTLS13,
		ServerName: serverName,
		RootCAs:    pool,
		GetClientCertificate: func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
			return f.keyPair()
		},
	}), nil
}

// PeerCert returns the verified certificate of the caller, or nil when the connection is not TLS.
func PeerCert(ctx context.Context) *x509.Certificate {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return nil
	}
	info, ok := p.AuthInfo.(credentials.TLSInfo)
	if !ok || len(info.State.PeerCertificates) == 0 {
		return nil
	}
	return info.State.PeerCertificates[0]
}

// PeerNames returns the DNS names in the verified certificate of the caller, or nil when the
// connection is not TLS.
func PeerNames(ctx context.Context) []string {
	if c := PeerCert(ctx); c != nil {
		return c.DNSNames
	}
	return nil
}

// Fingerprint is the SHA-256 of the certificate's DER encoding, in hex.
func Fingerprint(c *x509.Certificate) string {
	sum := sha256.Sum256(c.Raw)
	return hex.EncodeToString(sum[:])
}

// RequirePeer rejects every call whose certificate does not carry name. It keeps a runner's
// certificate, which the same CA signed, from being used to call another runner.
func RequirePeer(name string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, next grpc.UnaryHandler) (any, error) {
		if !slices.Contains(PeerNames(ctx), name) {
			return nil, status.Error(codes.PermissionDenied, "peer certificate not allowed")
		}
		return next(ctx, req)
	}
}

// CheckPeer tells whether the caller's certificate carries name.
func CheckPeer(ctx context.Context, name string) error {
	if !slices.Contains(PeerNames(ctx), name) {
		return status.Errorf(codes.PermissionDenied, "peer certificate does not carry %q", name)
	}
	return nil
}
