package tools

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequestErrorError(t *testing.T) {
	t.Run("reports the wrapped error", func(t *testing.T) {
		err := RequestError{Status: http.StatusBadRequest, Err: errors.New("bad input")}
		if got := err.Error(); got != "bad input" {
			t.Errorf("Error() = %q, want %q", got, "bad input")
		}
	})

	// Handlers routinely build a RequestError with only a status and a message; a nil
	// dereference here would panic inside the error path itself.
	t.Run("an error without a cause is empty rather than a panic", func(t *testing.T) {
		err := RequestError{Status: http.StatusForbidden, CustomMessage: "nope"}
		if got := err.Error(); got != "" {
			t.Errorf("Error() = %q, want empty", got)
		}
	})

	t.Run("satisfies the error interface by value", func(t *testing.T) {
		var _ error = RequestError{}
	})
}

func TestRequestErrorToResponse(t *testing.T) {
	tests := []struct {
		name string
		err  RequestError
		want gin.H
	}{
		{
			name: "carries status, message and cause",
			err:  RequestError{Status: http.StatusBadRequest, CustomMessage: "can not parse id", Err: errors.New("strconv: bad syntax")},
			want: gin.H{"status": http.StatusBadRequest, "message": "can not parse id", "error": "strconv: bad syntax"},
		},
		{
			// The internal cause must stay out of the body when there is none, so the
			// client is not handed an empty "error" key to branch on.
			name: "omits the error key when there is no cause",
			err:  RequestError{Status: http.StatusForbidden, CustomMessage: "forbidden"},
			want: gin.H{"status": http.StatusForbidden, "message": "forbidden"},
		},
		{
			name: "a zero value still renders both required keys",
			err:  RequestError{},
			want: gin.H{"status": 0, "message": ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.ToResponse()
			if len(got) != len(tt.want) {
				t.Fatalf("keys = %v, want %v", got, tt.want)
			}
			for k, want := range tt.want {
				if got[k] != want {
					t.Errorf("%q = %v, want %v", k, got[k], want)
				}
			}
		})
	}
}

// ErrorHandler is mounted in front of the API routes, so both error shapes and the
// no-error case are pinned end to end.
func TestErrorHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("renders a RequestError with its own status and body", func(t *testing.T) {
		rec := serveThroughErrorHandler(t, func(c *gin.Context) {
			_ = c.Error(RequestError{
				Status:        http.StatusNotFound,
				CustomMessage: "no such stream",
				Err:           errors.New("record not found"),
			})
		})

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
		}
		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("body %q is not JSON: %v", rec.Body.String(), err)
		}
		want := map[string]any{"status": float64(http.StatusNotFound), "message": "no such stream", "error": "record not found"}
		for k, v := range want {
			if body[k] != v {
				t.Errorf("body[%q] = %v, want %v", k, body[k], v)
			}
		}
	})

	t.Run("turns a plain error into a 500 with the bare message", func(t *testing.T) {
		rec := serveThroughErrorHandler(t, func(c *gin.Context) {
			_ = c.Error(errors.New("database exploded"))
		})

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
		}
		// The default branch serialises the raw message as a bare JSON string, not an
		// object, so clients cannot parse both shapes the same way.
		if got := rec.Body.String(); got != `"database exploded"` {
			t.Errorf("body = %q, want %q", got, `"database exploded"`)
		}
	})

	t.Run("only the first error decides the response", func(t *testing.T) {
		rec := serveThroughErrorHandler(t, func(c *gin.Context) {
			_ = c.Error(RequestError{Status: http.StatusTeapot, CustomMessage: "first"})
			_ = c.Error(errors.New("second"))
		})

		if rec.Code != http.StatusTeapot {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusTeapot)
		}
	})

	t.Run("leaves a successful response alone", func(t *testing.T) {
		rec := serveThroughErrorHandler(t, func(c *gin.Context) {
			c.String(http.StatusOK, "fine")
		})

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		if got := rec.Body.String(); got != "fine" {
			t.Errorf("body = %q, want %q", got, "fine")
		}
	})

	// A RequestError pushed by a handler that already wrote a body must not have its
	// status silently applied on top of the committed one.
	t.Run("an already written response keeps its status", func(t *testing.T) {
		rec := serveThroughErrorHandler(t, func(c *gin.Context) {
			c.String(http.StatusOK, "partial")
			_ = c.Error(RequestError{Status: http.StatusBadRequest, CustomMessage: "too late"})
		})

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
		}
	})
}

// serveThroughErrorHandler runs one request through ErrorHandler and the given handler.
func serveThroughErrorHandler(t *testing.T, handler gin.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()

	router := gin.New()
	router.Use(ErrorHandler)
	router.GET("/api/thing", handler)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/thing", nil))
	return rec
}
