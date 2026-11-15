package apiv2

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Bounds on a browser recording's layout. A chunk is two seconds of MediaRecorder
// output, so these are far above any lecture; they only keep names fixed-width and
// a malicious client from making a directory per possible number.
const (
	maxRecordingSegments = 1000
	maxRecordingChunks   = 1_000_000
	// maxRecordingChunkSize is well above a two-second chunk of a 4K screen.
	maxRecordingChunkSize = 16 << 20
)

var errRecordingLayout = errors.New("segment or chunk out of range")

// recordingStore keeps the chunks of browser recordings until they are handed to a
// runner: <root>/<recording id>/<segment>/<chunk>. Nothing about what was received is
// kept in memory or in the database, so a server that restarts during a lecture
// carries on from the directory.
//
// The chunks are on this server's own disk. Running a second replica of the server
// needs this moved to storage the replicas share (or each recording routed to one
// replica) first: otherwise a recording's chunks end up split between them, and the
// replica that finalizes it hands off an incomplete video.
type recordingStore struct {
	root string
}

func (s recordingStore) recordingDir(id uint) string {
	return filepath.Join(s.root, strconv.FormatUint(uint64(id), 10))
}

func (s recordingStore) segmentDir(id uint, segment uint32) string {
	return filepath.Join(s.recordingDir(id), fmt.Sprintf("%06d", segment))
}

func chunkName(seq uint32) string { return fmt.Sprintf("%08d", seq) }

// A chunk is stored in two steps, so that a chunk the browser was told about survives
// the server crashing right after: stage writes and syncs it to a temporary file,
// commit renames it into place and syncs the directory. Only commit has to happen while
// the recording is known to take chunks; reading a slow body must not hold that up.
// A chunk sent twice, because the answer to the first was lost, simply replaces itself.

// stagingDir holds chunks being received. Beside the recordings, on the same file
// system, so a commit is a rename; not in one, so removing a recording can't pull the
// directory away from a chunk being written.
func (s recordingStore) stagingDir() string { return filepath.Join(s.root, ".staging") }

// stage writes a chunk's body to a temporary file and syncs it.
func (s recordingStore) stage(body io.Reader) (string, error) {
	if err := os.MkdirAll(s.stagingDir(), 0o755); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(s.stagingDir(), "chunk-*")
	if err != nil {
		return "", err
	}
	_, err = io.Copy(tmp, body)
	if err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(tmp.Name())
		return "", err
	}
	return tmp.Name(), nil
}

// commit moves a staged chunk into the recording. The staged file is gone afterwards,
// whether or not it worked.
func (s recordingStore) commit(staged string, id uint, segment, seq uint32) error {
	defer func() { _ = os.Remove(staged) }() // a no-op once renamed
	if segment >= maxRecordingSegments || seq >= maxRecordingChunks {
		return errRecordingLayout
	}
	dir := s.segmentDir(id, segment)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.Rename(staged, filepath.Join(dir, chunkName(seq))); err != nil {
		return err
	}
	return syncDir(dir)
}

// sweepStaging removes staged chunks older than before: a crash while one was being
// received leaves it behind, and no request will commit it.
func (s recordingStore) sweepStaging(before time.Time) {
	entries, err := os.ReadDir(s.stagingDir())
	if err != nil {
		return
	}
	for _, e := range entries {
		if info, err := e.Info(); err == nil && info.ModTime().Before(before) {
			_ = os.Remove(filepath.Join(s.stagingDir(), e.Name()))
		}
	}
}

// gapName holds how long, in milliseconds, the recording was interrupted before a
// segment. Not a number, so received() doesn't take it for a chunk.
const gapName = "gap-ms"

// setGap records the interruption before a segment, synced like a chunk.
func (s recordingStore) setGap(id uint, segment uint32, ms int64) error {
	if segment >= maxRecordingSegments {
		return errRecordingLayout
	}
	dir := s.segmentDir(id, segment)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	staged, err := s.stage(strings.NewReader(strconv.FormatInt(ms, 10)))
	if err != nil {
		return err
	}
	if err := os.Rename(staged, filepath.Join(dir, gapName)); err != nil {
		_ = os.Remove(staged)
		return err
	}
	return syncDir(dir)
}

// gap is the interruption before a segment in milliseconds, 0 when none was recorded.
func (s recordingStore) gap(id uint, segment uint32) int64 {
	b, err := os.ReadFile(filepath.Join(s.segmentDir(id, segment), gapName))
	if err != nil {
		return 0
	}
	ms, _ := strconv.ParseInt(string(b), 10, 64)
	return max(ms, 0)
}

// put stages and commits a chunk in one go.
func (s recordingStore) put(id uint, segment, seq uint32, body io.Reader) error {
	staged, err := s.stage(body)
	if err != nil {
		return err
	}
	return s.commit(staged, id, segment, seq)
}

func syncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	err = d.Sync()
	if closeErr := d.Close(); err == nil {
		err = closeErr
	}
	return err
}

// received is, per segment from 0 on, how many chunks from 0 on are there without a
// gap. Only those make a playable file: a chunk after a gap is held back until the
// gap is filled. A segment with nothing is 0, and trailing ones are left out.
func (s recordingStore) received(id uint) ([]uint32, error) {
	entries, err := os.ReadDir(s.recordingDir(id))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var segments []uint32
	for _, e := range entries {
		n, err := strconv.ParseUint(e.Name(), 10, 32)
		if err != nil || !e.IsDir() || n >= maxRecordingSegments {
			continue
		}
		segments = append(segments, uint32(n))
	}
	if len(segments) == 0 {
		return nil, nil
	}
	sort.Slice(segments, func(i, j int) bool { return segments[i] < segments[j] })

	counts := make([]uint32, segments[len(segments)-1]+1)
	for _, seg := range segments {
		n, err := s.contiguousChunks(id, seg)
		if err != nil {
			return nil, err
		}
		counts[seg] = n
	}
	for len(counts) > 0 && counts[len(counts)-1] == 0 {
		counts = counts[:len(counts)-1]
	}
	return counts, nil
}

func (s recordingStore) contiguousChunks(id uint, segment uint32) (uint32, error) {
	entries, err := os.ReadDir(s.segmentDir(id, segment))
	if err != nil {
		return 0, err
	}
	present := make(map[uint32]bool, len(entries))
	for _, e := range entries {
		// Temporary files start with a dot and don't parse.
		if n, err := strconv.ParseUint(e.Name(), 10, 32); err == nil {
			present[uint32(n)] = true
		}
	}
	var n uint32
	for present[n] {
		n++
	}
	return n, nil
}

// segment reads the first `chunks` chunks of a segment as one file: the chunks of one
// MediaRecorder are a valid file when joined in order. Each chunk is opened only when
// it is reached, so a long segment does not hold a file descriptor per chunk.
func (s recordingStore) segment(id uint, segment, chunks uint32) io.ReadCloser {
	return &chunkReader{dir: s.segmentDir(id, segment), total: chunks}
}

// lastWrite is when the recording last took a chunk, or its directory was made.
func (s recordingStore) lastWrite(id uint) (time.Time, error) {
	var latest time.Time
	err := filepath.WalkDir(s.recordingDir(id), func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.ModTime().After(latest) {
			latest = info.ModTime()
		}
		return nil
	})
	return latest, err
}

func (s recordingStore) remove(id uint) error {
	return os.RemoveAll(s.recordingDir(id))
}

type chunkReader struct {
	dir   string
	total uint32
	next  uint32
	cur   *os.File
}

func (r *chunkReader) Read(p []byte) (int, error) {
	for {
		if r.cur == nil {
			if r.next >= r.total {
				return 0, io.EOF
			}
			f, err := os.Open(filepath.Join(r.dir, chunkName(r.next)))
			if err != nil {
				return 0, err
			}
			r.cur = f
			r.next++
		}
		n, err := r.cur.Read(p)
		if errors.Is(err, io.EOF) {
			_ = r.cur.Close()
			r.cur = nil
			if n > 0 {
				return n, nil
			}
			continue
		}
		return n, err
	}
}

func (r *chunkReader) Close() error {
	if r.cur == nil {
		return nil
	}
	err := r.cur.Close()
	r.cur = nil
	return err
}
