package apiv2

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// An upload ticket is the only credential of an upload that is not tied to a session:
// an integration's, or a browser recording's chunks. Only its hash is stored.

// newUploadTicket returns a fresh ticket as it goes into a path, and the hash to store.
func newUploadTicket() (raw string, hash []byte, err error) {
	ticket := make([]byte, 32)
	if _, err := rand.Read(ticket); err != nil {
		return "", nil, err
	}
	sum := sha256.Sum256(ticket)
	return base64.RawURLEncoding.EncodeToString(ticket), sum[:], nil
}

// uploadTicketHash is the stored hash of a ticket from a path, false for anything that
// is not one. A ticket has one spelling: base64 that decodes alike but reads otherwise
// is refused, so a path names a ticket in one way only.
func uploadTicketHash(raw string) ([]byte, bool) {
	ticket, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(ticket) != 32 || base64.RawURLEncoding.EncodeToString(ticket) != raw {
		return nil, false
	}
	sum := sha256.Sum256(ticket)
	return sum[:], true
}
