// Package mediareap works out which files on disk belong only to soft-deleted streams,
// so an admin can see them listed and then reap them.
//
// Soft-deleting a stream or course never touches disk. The hazard in reaping afterwards
// is that files are shared: copying a lecture or course copies the playlist URLs and
// File paths by value, so a copy keeps pointing at the *original's* files. Reaping is
// therefore a set difference: everything the deleted streams reference, minus
// everything any other stream still references.
package mediareap

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

// vodURL matches the playlist URLs the runner reports, https://<edge>/vod/<streamID>/
// STREAM_VERSION_<VERSION>/playlist.m3u8, which edge serves from <vodRoot>/<streamID>/
// <VERSION>/. Older shapes (/vod/<name>.mp4/, old edge hosts, external LRZ URLs) do not
// say reliably where their files are, or share a directory by name; they never match
// and are reported as skipped rather than guessed at.
var vodURL = regexp.MustCompile(`^https?://[^/]+/vod/([0-9]+)/STREAM_VERSION_([A-Z_]+)/playlist\.m3u8$`)

// Stream is the part of a stream the planner needs.
type Stream struct {
	ID        uint
	CourseID  uint
	DeletedAt *time.Time
	// PlaylistUrl, PlaylistUrlPRES and PlaylistUrlCAM.
	URLs [3]string
}

// File is a File row of a stream.
type File struct {
	StreamID uint
	Path     string
	// Deleted marks a soft-deleted File row: it no longer protects its path.
	Deleted bool
}

type Kind int

const (
	// KindVODDir is the directory of one stream version.
	KindVODDir Kind = iota + 1
	// KindVODFile is a file inside a KindVODDir.
	KindVODFile
	// KindFile is a File row's path: thumbnail, section image, attachment.
	KindFile
)

// Entry is one thing the reap removes.
type Entry struct {
	Kind      Kind
	Path      string
	Bytes     int64
	StreamIDs []uint
	CourseIDs []uint
	DeletedAt time.Time
}

// Skipped is something that looked reapable but is deliberately left alone.
type Skipped struct {
	Path      string
	StreamIDs []uint
	Reason    string
}

// Plan is everything a reap would remove, ordered by path (a directory sorts before its
// files). Token identifies exactly this content.
type Plan struct {
	Entries []Entry
	Skipped []Skipped
	Dirs    int
	Files   int
	Bytes   int64
	Token   string
}

// Options say where files live and which streams count as reapable.
type Options struct {
	// VODRoot holds <streamID>/<VERSION>/ directories. Empty: VOD directories are not
	// planned.
	VODRoot string
	// MassRoot bounds File paths: nothing outside it is ever listed.
	MassRoot string
	// Cutoff: only streams soft-deleted before it are reapable, so un-deleting a course
	// shortly after stays safe.
	Cutoff time.Time
}

type dirKey struct{ id, version string }

func (k dirKey) path(root string) string { return filepath.Join(root, k.id, k.version) }

type owners struct {
	streams []uint
	courses []uint
	deleted time.Time
}

func (o *owners) add(s Stream) {
	if !slices.Contains(o.streams, s.ID) {
		o.streams = append(o.streams, s.ID)
	}
	if !slices.Contains(o.courses, s.CourseID) {
		o.courses = append(o.courses, s.CourseID)
	}
	if s.DeletedAt != nil && s.DeletedAt.After(o.deleted) {
		o.deleted = *s.DeletedAt
	}
}

// Build computes the plan. It only reads: the filesystem is inspected, never changed.
func Build(streams []Stream, files []File, opt Options) (*Plan, error) {
	candidate := map[uint]Stream{}
	for _, s := range streams {
		if s.DeletedAt != nil && s.DeletedAt.Before(opt.Cutoff) {
			candidate[s.ID] = s
		}
	}

	plan := &Plan{}

	// What every other stream still uses. Live streams, and ones deleted too recently,
	// protect their files even when a deleted stream names the same ones.
	keptDirs := map[dirKey][]uint{}
	for _, s := range streams {
		if _, ok := candidate[s.ID]; ok {
			continue
		}
		for _, u := range s.URLs {
			if k, ok := parseVOD(u); ok {
				keptDirs[k] = append(keptDirs[k], s.ID)
			}
		}
	}
	// A stream keeps the directories named after its own ID even while its URL is blank
	// (still transcoding), which a copy's URL may already point at.
	keptIDs := map[string]uint{}
	for _, s := range streams {
		if _, ok := candidate[s.ID]; !ok {
			keptIDs[fmt.Sprint(s.ID)] = s.ID
		}
	}
	keptFiles := map[string][]uint{}
	for _, f := range files {
		if _, ok := candidate[f.StreamID]; ok || f.Deleted {
			continue
		}
		keptFiles[filepath.Clean(f.Path)] = append(keptFiles[filepath.Clean(f.Path)], f.StreamID)
	}

	// Candidate streams, in ID order so the plan (and its token) is deterministic.
	ids := make([]uint, 0, len(candidate))
	for id := range candidate {
		ids = append(ids, id)
	}
	slices.Sort(ids)

	dirs := map[dirKey]*owners{}
	for _, id := range ids {
		s := candidate[id]
		for _, u := range s.URLs {
			if u == "" {
				continue
			}
			k, ok := parseVOD(u)
			if !ok {
				plan.Skipped = append(plan.Skipped, Skipped{Path: u, StreamIDs: []uint{s.ID}, Reason: "not a current-format VOD URL, location unknown"})
				continue
			}
			if opt.VODRoot == "" {
				continue
			}
			if dirs[k] == nil {
				dirs[k] = &owners{}
			}
			dirs[k].add(s)
		}
	}

	filePaths := map[string]*owners{}
	for _, f := range files {
		s, ok := candidate[f.StreamID]
		if !ok {
			continue
		}
		p := filepath.Clean(f.Path)
		switch {
		case f.Path == "" || strings.HasPrefix(f.Path, "http://") || strings.HasPrefix(f.Path, "https://"):
			continue
		case opt.VODRoot != "" && within(opt.VODRoot, p):
			// Already listed with its directory; a second entry would double-count it.
			continue
		case opt.MassRoot == "" || !within(opt.MassRoot, p):
			plan.Skipped = append(plan.Skipped, Skipped{Path: f.Path, StreamIDs: []uint{s.ID}, Reason: "outside the mass storage directory"})
			continue
		}
		if filePaths[p] == nil {
			filePaths[p] = &owners{}
		}
		filePaths[p].add(s)
	}

	dirKeys := make([]dirKey, 0, len(dirs))
	for k := range dirs {
		dirKeys = append(dirKeys, k)
	}
	slices.SortFunc(dirKeys, func(a, b dirKey) int { return strings.Compare(a.path(""), b.path("")) })

	for _, k := range dirKeys {
		o := dirs[k]
		p := k.path(opt.VODRoot)
		info, err := os.Lstat(p)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		by := keptDirs[k]
		if id, ok := keptIDs[k.id]; ok {
			by = append(by, id)
		}
		if len(by) > 0 {
			plan.Skipped = append(plan.Skipped, Skipped{Path: p, StreamIDs: o.streams, Reason: fmt.Sprintf("still used by stream %s", idList(by))})
			continue
		}
		if info.Mode()&fs.ModeSymlink != 0 || !info.IsDir() {
			plan.Skipped = append(plan.Skipped, Skipped{Path: p, StreamIDs: o.streams, Reason: "not a plain directory"})
			continue
		}
		plan.Entries = append(plan.Entries, Entry{Kind: KindVODDir, Path: p, StreamIDs: o.streams, CourseIDs: o.courses, DeletedAt: o.deleted})
		err = filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if path == p {
				return nil
			}
			e := Entry{Path: path, StreamIDs: o.streams, CourseIDs: o.courses, DeletedAt: o.deleted}
			if d.IsDir() {
				// A nested directory is removed when its files are; it is listed so the
				// admin sees it.
				e.Kind = KindVODDir
			} else {
				e.Kind = KindVODFile
				if fi, err := d.Info(); err == nil {
					e.Bytes = fi.Size()
				}
			}
			plan.Entries = append(plan.Entries, e)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	paths := make([]string, 0, len(filePaths))
	for p := range filePaths {
		paths = append(paths, p)
	}
	slices.Sort(paths)
	for _, p := range paths {
		o := filePaths[p]
		info, err := os.Lstat(p)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if by := keptFiles[p]; len(by) > 0 {
			plan.Skipped = append(plan.Skipped, Skipped{Path: p, StreamIDs: o.streams, Reason: fmt.Sprintf("still used by stream %s", idList(by))})
			continue
		}
		if !info.Mode().IsRegular() {
			plan.Skipped = append(plan.Skipped, Skipped{Path: p, StreamIDs: o.streams, Reason: "not a regular file"})
			continue
		}
		if !resolvedWithin(opt.MassRoot, p) {
			plan.Skipped = append(plan.Skipped, Skipped{Path: p, StreamIDs: o.streams, Reason: "resolves outside the mass storage directory"})
			continue
		}
		plan.Entries = append(plan.Entries, Entry{Kind: KindFile, Path: p, Bytes: info.Size(), StreamIDs: o.streams, CourseIDs: o.courses, DeletedAt: o.deleted})
	}

	slices.SortFunc(plan.Entries, func(a, b Entry) int { return strings.Compare(a.Path, b.Path) })
	h := sha256.New()
	for _, e := range plan.Entries {
		fmt.Fprintf(h, "%d\x00%s\x00%d\n", e.Kind, e.Path, e.Bytes)
		if e.Kind == KindVODDir {
			plan.Dirs++
		} else {
			plan.Files++
			plan.Bytes += e.Bytes
		}
	}
	plan.Token = hex.EncodeToString(h.Sum(nil))
	return plan, nil
}

// Result is what Execute removed.
type Result struct {
	Files  int
	Bytes  int64
	Errors []string
}

// Execute removes exactly the entries of the plan: each listed file, then the listed
// directories once they are empty. A directory that gained files since the plan was
// built is not emptied (os.Remove refuses), so nothing unreviewed is deleted.
func Execute(plan *Plan) Result {
	var res Result
	for _, e := range plan.Entries {
		if e.Kind == KindVODDir {
			continue
		}
		if err := os.Remove(e.Path); err != nil && !os.IsNotExist(err) {
			res.Errors = append(res.Errors, err.Error())
			continue
		}
		res.Files++
		res.Bytes += e.Bytes
	}
	// Deepest first, so nested directories go before their parents.
	for i := len(plan.Entries) - 1; i >= 0; i-- {
		e := plan.Entries[i]
		if e.Kind != KindVODDir {
			continue
		}
		if err := os.Remove(e.Path); err != nil && !os.IsNotExist(err) {
			res.Errors = append(res.Errors, err.Error())
			continue
		}
		// <id>/ stays behind empty once its last version goes; a non-empty one refuses.
		_ = os.Remove(filepath.Dir(e.Path))
	}
	return res
}

func parseVOD(u string) (dirKey, bool) {
	m := vodURL.FindStringSubmatch(u)
	if m == nil {
		return dirKey{}, false
	}
	return dirKey{id: m[1], version: m[2]}, true
}

// within reports whether p is strictly below root, lexically.
func within(root, p string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), p)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// resolvedWithin is within after resolving symlinks in the file's directory, so a
// symlinked directory inside the mass storage cannot point a path elsewhere.
func resolvedWithin(root, p string) bool {
	r, err := filepath.EvalSymlinks(root)
	if err != nil {
		return false
	}
	d, err := filepath.EvalSymlinks(filepath.Dir(p))
	if err != nil {
		return false
	}
	return within(r, filepath.Join(d, filepath.Base(p)))
}

func idList(ids []uint) string {
	slices.Sort(ids)
	ids = slices.Compact(ids)
	parts := make([]string, 0, 4)
	for i, id := range ids {
		if i == 3 {
			parts = append(parts, fmt.Sprintf("and %d more", len(ids)-3))
			break
		}
		parts = append(parts, fmt.Sprint(id))
	}
	return strings.Join(parts, ", ")
}
