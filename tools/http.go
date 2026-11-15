package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
)

// Middleware wraps a handler. It ends a request by not calling next, which is all
// "aborting" means here.
type Middleware func(http.Handler) http.Handler

type tumLiveContextKey struct{}

// WithTUMLiveContext returns r carrying c, replacing any context an earlier
// middleware set. InitContext sets the first one; InitCourse and InitStream replace it
// with the course and stream they found.
func WithTUMLiveContext(r *http.Request, c TUMLiveContext) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), tumLiveContextKey{}, c))
}

// GetTUMLiveContext returns the context InitContext set on r. It is missing on the
// static paths InitContext skips and wherever a route was registered without it.
func GetTUMLiveContext(r *http.Request) (TUMLiveContext, bool) {
	c, ok := r.Context().Value(tumLiveContextKey{}).(TUMLiveContext)
	return c, ok
}

// requireTUMLiveContext is GetTUMLiveContext for middleware that cannot do without:
// it answers 500 itself when the context is missing, because that is a route wired
// without InitContext, not something the caller did.
func requireTUMLiveContext(w http.ResponseWriter, r *http.Request) (TUMLiveContext, bool) {
	c, ok := GetTUMLiveContext(r)
	if !ok {
		logger.Error("context should exist but doesn't")
		w.WriteHeader(http.StatusInternalServerError)
	}
	return c, ok
}

// WriteJSON answers with v as JSON. Marshalled up front, so a value that cannot be
// encoded is a 500 rather than a truncated body behind a 200.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		logger.Error("can't encode response", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	WriteData(w, status, "application/json; charset=utf-8", body)
}

// WriteText answers with a plain-text body.
func WriteText(w http.ResponseWriter, status int, text string) {
	WriteData(w, status, "text/plain; charset=utf-8", []byte(text))
}

// WriteData answers with body as the given content type.
func WriteData(w http.ResponseWriter, status int, contentType string, body []byte) {
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

// SetCookie sets an HttpOnly cookie for the whole site. The value is query-escaped and
// Cookie unescapes it, as the cookies of the gin era were, so a cookie set before the
// migration still reads back (redirURL holds a path with slashes and a query).
func SetCookie(w http.ResponseWriter, name, value string, maxAge int, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    url.QueryEscape(value),
		MaxAge:   maxAge,
		Path:     "/",
		Secure:   secure,
		HttpOnly: true,
	})
}

// Cookie returns the unescaped value of the named cookie.
func Cookie(r *http.Request, name string) (string, error) {
	cookie, err := r.Cookie(name)
	if err != nil {
		return "", err
	}
	return url.QueryUnescape(cookie.Value)
}
