package apiv2

import (
	"net/http"
	"net/http/httptest"
)

// routed reports what parse makes of a request once pattern has routed it, and false
// when the pattern does not take the request at all. It is how the route tests ask "is
// this the endpoint's?" the way the server's mux does, rather than by a hand-written
// matcher.
func routed[T any](pattern, method, path string, parse func(*http.Request) (T, bool)) (T, bool) {
	var (
		got T
		ok  bool
	)
	mux := http.NewServeMux()
	mux.HandleFunc(pattern, func(_ http.ResponseWriter, r *http.Request) { got, ok = parse(r) })
	mux.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(method, path, nil))
	return got, ok
}
