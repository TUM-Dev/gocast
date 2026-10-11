package tools

import "crypto/rsa"

// SetJWTKeyForTest installs the key playlists and tokens are signed with, for tests
// of code that signs without booting the configuration. It returns what undoes it.
func SetJWTKeyForTest(k *rsa.PrivateKey) (restore func()) {
	prev := jwtKey
	jwtKey = k
	return func() { jwtKey = prev }
}
