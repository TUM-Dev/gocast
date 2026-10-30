package testutils

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/TUM-Dev/gocast/tools"
)

func Equal(t *testing.T, a, b interface{}) {
	// Compare JSON payloads semantically: search hits are decoded into maps, so a
	// byte-wise comparison would depend on the marshaller's key order.
	av, okA := a.([]byte)
	bv, okB := b.([]byte)

	if okA && okB && json.Valid(av) && json.Valid(bv) {
		assert.JSONEq(t, string(av), string(bv))
		return
	}

	assert.Equal(t, a, b)
}

// InjectContext is a middleware standing in for tools.InitContext: it puts ctx on every
// request, so a test can pick who is signed in without a session cookie or a database.
func InjectContext(ctx tools.TUMLiveContext) tools.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, tools.WithTUMLiveContext(r, ctx))
		})
	}
}
