package tools

import (
	"errors"
	"net/http"
	"testing"
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
		want map[string]any
	}{
		{
			name: "carries status, message and cause",
			err:  RequestError{Status: http.StatusBadRequest, CustomMessage: "can not parse id", Err: errors.New("strconv: bad syntax")},
			want: map[string]any{"status": http.StatusBadRequest, "message": "can not parse id", "error": "strconv: bad syntax"},
		},
		{
			// The internal cause must stay out of the body when there is none, so the
			// client is not handed an empty "error" key to branch on.
			name: "omits the error key when there is no cause",
			err:  RequestError{Status: http.StatusForbidden, CustomMessage: "forbidden"},
			want: map[string]any{"status": http.StatusForbidden, "message": "forbidden"},
		},
		{
			name: "a zero value still renders both required keys",
			err:  RequestError{},
			want: map[string]any{"status": 0, "message": ""},
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
