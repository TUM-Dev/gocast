package tools

import (
	"net/http"
	"slices"
	"strings"
)

// Router is an http.ServeMux with middleware groups, which is all of gin the site used.
// Patterns are ServeMux's: "GET /w/{slug}/{streamID}", read back with r.PathValue.
//
// Middleware applies to the routes registered after it, in the order it was added,
// and a group starts from a copy of its parent's: the same rules as gin's, so moving
// a route between groups changes what runs in front of it exactly as it did.
type Router struct {
	mux *http.ServeMux
	mws []Middleware
}

func NewRouter() *Router {
	return &Router{mux: http.NewServeMux()}
}

// Use adds middleware for the routes registered from now on.
func (r *Router) Use(mw ...Middleware) {
	r.mws = append(r.mws, mw...)
}

// Group returns a router on the same mux whose routes also run the given middleware.
// Middleware added to the group later does not reach the parent's routes.
func (r *Router) Group(mw ...Middleware) *Router {
	return &Router{mux: r.mux, mws: append(slices.Clone(r.mws), mw...)}
}

// Handle registers h behind the router's middleware. Like ServeMux it panics when
// the pattern conflicts with an earlier one, at startup rather than on a request.
func (r *Router) Handle(pattern string, h http.Handler) {
	for _, mw := range slices.Backward(r.mws) {
		h = mw(h)
	}
	r.mux.Handle(pattern, h)
}

func (r *Router) HandleFunc(pattern string, h http.HandlerFunc) {
	r.Handle(pattern, h)
}

// NotFound serves h, behind the router's middleware, for every request no other
// pattern takes. A path that only differs from a route by a trailing slash is
// redirected to the route first, as gin did: 301 for GET, 307 for the rest so a form
// post is not turned into a GET.
func (r *Router) NotFound(h http.Handler) {
	r.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if r.redirectTrailingSlash(w, req) {
			return
		}
		h.ServeHTTP(w, req)
	}))
}

func (r *Router) redirectTrailingSlash(w http.ResponseWriter, req *http.Request) bool {
	if len(req.URL.Path) < 2 || !strings.HasSuffix(req.URL.Path, "/") {
		return false
	}
	target := *req.URL
	target.Path = strings.TrimSuffix(target.Path, "/")
	target.RawPath = ""

	probe := req.Clone(req.Context())
	probe.URL = &target
	// "/" is NotFound's own pattern: matching it means the trimmed path has no route.
	if _, pattern := r.mux.Handler(probe); pattern == "" || pattern == "/" {
		return false
	}

	code := http.StatusMovedPermanently
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		code = http.StatusTemporaryRedirect
	}
	http.Redirect(w, req, target.String(), code)
	return true
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mux.ServeHTTP(w, req)
}
