package apiv2

import (
	"io"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestRecordingStore(t *testing.T) {
	put := func(t *testing.T, s recordingStore, segment, seq uint32, body string) {
		t.Helper()
		if err := s.put(7, segment, seq, strings.NewReader(body)); err != nil {
			t.Fatalf("put(%d, %d): %v", segment, seq, err)
		}
	}
	read := func(t *testing.T, s recordingStore, segment, chunks uint32) string {
		t.Helper()
		r := s.segment(7, segment, chunks)
		defer func() { _ = r.Close() }()
		b, err := io.ReadAll(r)
		if err != nil {
			t.Fatalf("read segment %d: %v", segment, err)
		}
		return string(b)
	}

	t.Run("joins a segment's chunks in order", func(t *testing.T) {
		s := recordingStore{root: t.TempDir()}
		put(t, s, 0, 1, "b")
		put(t, s, 0, 0, "a")
		put(t, s, 0, 2, "c")
		if got := read(t, s, 0, 3); got != "abc" {
			t.Errorf("segment = %q, want %q", got, "abc")
		}
	})

	t.Run("a chunk sent again replaces itself", func(t *testing.T) {
		s := recordingStore{root: t.TempDir()}
		put(t, s, 0, 0, "a")
		put(t, s, 0, 0, "a")
		if got, _ := s.received(7); !slices.Equal(got, []uint32{1}) {
			t.Errorf("received = %v, want [1]", got)
		}
		if got := read(t, s, 0, 1); got != "a" {
			t.Errorf("segment = %q, want %q", got, "a")
		}
	})

	t.Run("counts chunks only up to the first gap", func(t *testing.T) {
		s := recordingStore{root: t.TempDir()}
		put(t, s, 0, 0, "a")
		put(t, s, 0, 1, "b")
		put(t, s, 0, 3, "d")
		put(t, s, 2, 0, "x")
		got, err := s.received(7)
		if err != nil {
			t.Fatal(err)
		}
		if want := []uint32{2, 0, 1}; !slices.Equal(got, want) {
			t.Errorf("received = %v, want %v", got, want)
		}
	})

	t.Run("a staged chunk is not received until committed", func(t *testing.T) {
		s := recordingStore{root: t.TempDir()}
		staged, err := s.stage(strings.NewReader("a"))
		if err != nil {
			t.Fatal(err)
		}
		if got, _ := s.received(7); got != nil {
			t.Errorf("received before commit = %v", got)
		}
		if err := s.commit(staged, 7, 0, 0); err != nil {
			t.Fatal(err)
		}
		if got, _ := s.received(7); !slices.Equal(got, []uint32{1}) {
			t.Errorf("received = %v, want [1]", got)
		}
		if _, err := os.Stat(staged); !os.IsNotExist(err) {
			t.Errorf("staged file left: %v", err)
		}
	})

	t.Run("removing a recording leaves a chunk being staged alone", func(t *testing.T) {
		s := recordingStore{root: t.TempDir()}
		put(t, s, 0, 0, "a")
		staged, err := s.stage(strings.NewReader("b"))
		if err != nil {
			t.Fatal(err)
		}
		if err := s.remove(7); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(staged); err != nil {
			t.Errorf("staged chunk gone with the recording: %v", err)
		}
	})

	t.Run("sweeps staged chunks a crash left behind, not ones arriving", func(t *testing.T) {
		s := recordingStore{root: t.TempDir()}
		old, _ := s.stage(strings.NewReader("old"))
		fresh, _ := s.stage(strings.NewReader("fresh"))
		long := time.Now().Add(-2 * time.Hour)
		_ = os.Chtimes(old, long, long)
		s.sweepStaging(time.Now().Add(-time.Hour))
		if _, err := os.Stat(old); !os.IsNotExist(err) {
			t.Errorf("old staged chunk kept: %v", err)
		}
		if _, err := os.Stat(fresh); err != nil {
			t.Errorf("fresh staged chunk removed: %v", err)
		}
	})

	t.Run("nothing received is no segments", func(t *testing.T) {
		s := recordingStore{root: t.TempDir()}
		got, err := s.received(7)
		if err != nil || got != nil {
			t.Errorf("received = %v, %v; want nil, nil", got, err)
		}
	})

	t.Run("refuses numbers out of range", func(t *testing.T) {
		s := recordingStore{root: t.TempDir()}
		if err := s.put(7, maxRecordingSegments, 0, strings.NewReader("a")); err == nil {
			t.Error("put accepted a segment out of range")
		}
		if err := s.put(7, 0, maxRecordingChunks, strings.NewReader("a")); err == nil {
			t.Error("put accepted a chunk out of range")
		}
	})

	t.Run("remove drops everything", func(t *testing.T) {
		s := recordingStore{root: t.TempDir()}
		put(t, s, 0, 0, "a")
		if err := s.remove(7); err != nil {
			t.Fatal(err)
		}
		if got, _ := s.received(7); got != nil {
			t.Errorf("received after remove = %v", got)
		}
	})
}
