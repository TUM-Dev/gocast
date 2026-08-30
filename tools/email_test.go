package tools

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

// Failures used to be appended to a varchar(191) until the row became unwritable and
// the send was retried forever. The column is a longtext now; this bounds the rest.
func TestAppendSendError(t *testing.T) {
	t.Run("keeps every attempt while they fit", func(t *testing.T) {
		var stored string
		for i := range 11 {
			stored = appendSendError(stored, fmt.Errorf("attempt %d failed", i))
		}

		if lines := strings.Count(stored, "\n"); lines != 11 {
			t.Errorf("recorded %d attempts, want 11", lines)
		}
		if strings.Contains(stored, truncationMark) {
			t.Error("truncated a set of errors that fit")
		}
	})

	t.Run("stays within the bound when they do not", func(t *testing.T) {
		var stored string
		for i := range 50 {
			stored = appendSendError(stored, fmt.Errorf("%d: %s", i, strings.Repeat("x", 500)))
		}

		if len(stored) > maxStoredErrors+len(truncationMark) {
			t.Errorf("stored %d bytes, want at most %d", len(stored), maxStoredErrors+len(truncationMark))
		}
	})

	t.Run("keeps the most recent attempt", func(t *testing.T) {
		// The reason to read this field is why it is failing now.
		var stored string
		for i := range 50 {
			stored = appendSendError(stored, fmt.Errorf("%d: %s", i, strings.Repeat("x", 500)))
		}

		if !strings.Contains(stored, "49: ") {
			t.Error("dropped the latest attempt")
		}
		if strings.Contains(stored, "0: ") {
			t.Error("kept the oldest attempt over the latest")
		}
	})

	t.Run("says that it dropped something", func(t *testing.T) {
		// Otherwise a first line cut in half reads as a mangled error.
		stored := appendSendError(strings.Repeat("old error\n", 1000), errors.New("newest"))

		if !strings.HasPrefix(stored, truncationMark) {
			t.Errorf("truncated without saying so, got %q", stored[:40])
		}
	})

	t.Run("resumes on a line boundary", func(t *testing.T) {
		stored := appendSendError(strings.Repeat("old error\n", 1000), errors.New("newest"))

		body := strings.TrimPrefix(stored, truncationMark)
		if !strings.HasPrefix(body, "old error") {
			t.Errorf("first kept line is a fragment: %q", body[:20])
		}
	})

	t.Run("does not cut a multi-byte character in half", func(t *testing.T) {
		// Half a rune in a utf8mb4 column fails on write rather than here.
		var stored string
		for range 50 {
			stored = appendSendError(stored, errors.New(strings.Repeat("über—weit", 200)))
		}

		if !utf8.ValidString(stored) {
			t.Error("stored invalid UTF-8")
		}
	})

	t.Run("survives one error larger than the whole bound", func(t *testing.T) {
		// No line break to resume at.
		stored := appendSendError("", errors.New(strings.Repeat("ü", maxStoredErrors)))

		if len(stored) > maxStoredErrors+len(truncationMark) {
			t.Errorf("stored %d bytes, want at most %d", len(stored), maxStoredErrors+len(truncationMark))
		}
		if !utf8.ValidString(stored) {
			t.Error("stored invalid UTF-8")
		}
	})
}
