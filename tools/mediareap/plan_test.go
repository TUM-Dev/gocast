package mediareap

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

var now = time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)

func ago(d time.Duration) *time.Time {
	t := now.Add(-d)
	return &t
}

const month = 30 * 24 * time.Hour

func vod(id string) string {
	return "https://edge.live.rbg.tum.de/vod/" + id + "/STREAM_VERSION_COMBINED/playlist.m3u8"
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func paths(p *Plan) []string {
	out := make([]string, 0, len(p.Entries))
	for _, e := range p.Entries {
		out = append(out, e.Path)
	}
	return out
}

func setup(t *testing.T) (vodRoot, massRoot string, opt Options) {
	t.Helper()
	root := t.TempDir()
	vodRoot = filepath.Join(root, "vod")
	massRoot = filepath.Join(root, "mass")
	for _, d := range []string{vodRoot, massRoot} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return vodRoot, massRoot, Options{VODRoot: vodRoot, MassRoot: massRoot, Cutoff: now.Add(-month)}
}

func TestListsEveryFileOfAReapableStream(t *testing.T) {
	vodRoot, _, opt := setup(t)
	write(t, filepath.Join(vodRoot, "10", "COMBINED", "playlist.m3u8"), "abc")
	write(t, filepath.Join(vodRoot, "10", "COMBINED", "00000.ts"), "12345")

	plan, err := Build([]Stream{{ID: 10, CourseID: 1, DeletedAt: ago(2 * month), URLs: [3]string{vod("10")}}}, nil, opt)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{
		filepath.Join(vodRoot, "10", "COMBINED"),
		filepath.Join(vodRoot, "10", "COMBINED", "00000.ts"),
		filepath.Join(vodRoot, "10", "COMBINED", "playlist.m3u8"),
	}
	if got := paths(plan); !slices.Equal(got, want) {
		t.Fatalf("entries = %v, want %v", got, want)
	}
	if plan.Dirs != 1 || plan.Files != 2 || plan.Bytes != 8 {
		t.Errorf("dirs, files, bytes = %d, %d, %d, want 1, 2, 8", plan.Dirs, plan.Files, plan.Bytes)
	}
}

func TestDryRunChangesNothing(t *testing.T) {
	vodRoot, _, opt := setup(t)
	f := filepath.Join(vodRoot, "10", "COMBINED", "00000.ts")
	write(t, f, "x")

	if _, err := Build([]Stream{{ID: 10, DeletedAt: ago(2 * month), URLs: [3]string{vod("10")}}}, nil, opt); err != nil {
		t.Fatal(err)
	}
	if !exists(f) {
		t.Fatal("building a plan removed a file")
	}
}

// A copy keeps the original's URL. Deleting the original must not take the copy's video.
func TestALiveCopyProtectsADeletedOriginalsFiles(t *testing.T) {
	vodRoot, _, opt := setup(t)
	write(t, filepath.Join(vodRoot, "10", "COMBINED", "playlist.m3u8"), "x")
	streams := []Stream{
		{ID: 10, CourseID: 1, DeletedAt: ago(2 * month), URLs: [3]string{vod("10")}},
		{ID: 20, CourseID: 2, URLs: [3]string{vod("10")}}, // a copy
	}

	plan, err := Build(streams, nil, opt)
	if err != nil {
		t.Fatal(err)
	}

	if len(plan.Entries) != 0 {
		t.Fatalf("entries = %v, want none", paths(plan))
	}
	if len(plan.Skipped) != 1 || plan.Skipped[0].Reason != "still used by stream 20" {
		t.Errorf("skipped = %+v, want the dir kept for stream 20", plan.Skipped)
	}
}

// Deleted recently enough to still be restored counts as in use.
func TestACopyInTheGracePeriodProtectsToo(t *testing.T) {
	vodRoot, _, opt := setup(t)
	write(t, filepath.Join(vodRoot, "10", "COMBINED", "playlist.m3u8"), "x")
	streams := []Stream{
		{ID: 10, DeletedAt: ago(2 * month), URLs: [3]string{vod("10")}},
		{ID: 20, DeletedAt: ago(time.Hour), URLs: [3]string{vod("10")}},
	}

	plan, err := Build(streams, nil, opt)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Entries) != 0 {
		t.Fatalf("entries = %v, want none", paths(plan))
	}
}

func TestOnceEveryReferenceIsDeletedTheDirIsListedOnceForAll(t *testing.T) {
	vodRoot, _, opt := setup(t)
	write(t, filepath.Join(vodRoot, "10", "COMBINED", "playlist.m3u8"), "x")
	streams := []Stream{
		{ID: 10, CourseID: 1, DeletedAt: ago(2 * month), URLs: [3]string{vod("10")}},
		{ID: 20, CourseID: 2, DeletedAt: ago(3 * month), URLs: [3]string{vod("10")}},
	}

	plan, err := Build(streams, nil, opt)
	if err != nil {
		t.Fatal(err)
	}

	if plan.Dirs != 1 {
		t.Fatalf("dirs = %d, want 1", plan.Dirs)
	}
	if !slices.Equal(plan.Entries[0].StreamIDs, []uint{10, 20}) {
		t.Errorf("owners = %v, want both copies", plan.Entries[0].StreamIDs)
	}
}

func TestTheGracePeriodIsRespected(t *testing.T) {
	vodRoot, _, opt := setup(t)
	write(t, filepath.Join(vodRoot, "10", "COMBINED", "playlist.m3u8"), "x")

	plan, err := Build([]Stream{{ID: 10, DeletedAt: ago(month - time.Hour), URLs: [3]string{vod("10")}}}, nil, opt)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Entries) != 0 {
		t.Fatalf("entries = %v, want none inside the grace period", paths(plan))
	}
}

func TestOtherURLShapesAreSkippedNotGuessed(t *testing.T) {
	vodRoot, _, opt := setup(t)
	write(t, filepath.Join(vodRoot, "x_2023.mp4", "playlist.m3u8"), "x")
	streams := []Stream{{ID: 10, DeletedAt: ago(2 * month), URLs: [3]string{
		"https://edge.live.rbg.tum.de/vod/x_2023.mp4/playlist.m3u8",
		"https://live.stream.lrz.de/livetum/smil:_all.smil/playlist.m3u8",
		"https://edge.live.rbg.tum.de/vod/../../etc/STREAM_VERSION_COMBINED/playlist.m3u8",
	}}}

	plan, err := Build(streams, nil, opt)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Entries) != 0 || len(plan.Skipped) != 3 {
		t.Fatalf("entries = %v, skipped = %d, want nothing planned and 3 skipped", paths(plan), len(plan.Skipped))
	}
}

func TestFilesAreProtectedByTheirPathNotTheirRow(t *testing.T) {
	_, massRoot, opt := setup(t)
	shared := filepath.Join(massRoot, "thumbs", "a.jpg")
	own := filepath.Join(massRoot, "thumbs", "b.jpg")
	write(t, shared, "x")
	write(t, own, "yy")
	streams := []Stream{
		{ID: 10, CourseID: 1, DeletedAt: ago(2 * month)},
		{ID: 20, CourseID: 2}, // a copy sharing a.jpg
	}
	files := []File{
		{StreamID: 10, Path: shared},
		{StreamID: 10, Path: own},
		{StreamID: 20, Path: shared},
	}

	plan, err := Build(streams, files, opt)
	if err != nil {
		t.Fatal(err)
	}

	if got := paths(plan); !slices.Equal(got, []string{own}) {
		t.Fatalf("entries = %v, want only the unshared file", got)
	}
	if len(plan.Skipped) != 1 || plan.Skipped[0].Path != shared {
		t.Errorf("skipped = %+v, want the shared file", plan.Skipped)
	}
}

func TestASoftDeletedFileRowDoesNotProtectItsPath(t *testing.T) {
	_, massRoot, opt := setup(t)
	p := filepath.Join(massRoot, "a.jpg")
	write(t, p, "x")
	streams := []Stream{{ID: 10, DeletedAt: ago(2 * month)}, {ID: 20}}
	files := []File{{StreamID: 10, Path: p}, {StreamID: 20, Path: p, Deleted: true}}

	plan, err := Build(streams, files, opt)
	if err != nil {
		t.Fatal(err)
	}
	if got := paths(plan); !slices.Equal(got, []string{p}) {
		t.Fatalf("entries = %v, want the file", got)
	}
}

func TestPathsOutsideTheMassStorageAreNeverListed(t *testing.T) {
	_, massRoot, opt := setup(t)
	outside := filepath.Join(filepath.Dir(massRoot), "secret.txt")
	write(t, outside, "x")
	streams := []Stream{{ID: 10, DeletedAt: ago(2 * month)}}
	files := []File{
		{StreamID: 10, Path: outside},
		{StreamID: 10, Path: filepath.Join(massRoot, "..", "secret.txt")},
		{StreamID: 10, Path: massRoot},
		{StreamID: 10, Path: "https://example.org/a.pdf"},
	}

	plan, err := Build(streams, files, opt)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Entries) != 0 {
		t.Fatalf("entries = %v, want none", paths(plan))
	}
}

func TestASymlinkedDirectoryCannotPointAFileElsewhere(t *testing.T) {
	_, massRoot, opt := setup(t)
	elsewhere := t.TempDir()
	write(t, filepath.Join(elsewhere, "a.jpg"), "x")
	if err := os.Symlink(elsewhere, filepath.Join(massRoot, "link")); err != nil {
		t.Skip("no symlinks here")
	}
	streams := []Stream{{ID: 10, DeletedAt: ago(2 * month)}}
	files := []File{{StreamID: 10, Path: filepath.Join(massRoot, "link", "a.jpg")}}

	plan, err := Build(streams, files, opt)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Entries) != 0 {
		t.Fatalf("entries = %v, want none", paths(plan))
	}
}

func TestAFileThatIsGoneIsNotListed(t *testing.T) {
	_, massRoot, opt := setup(t)
	streams := []Stream{{ID: 10, DeletedAt: ago(2 * month)}}
	files := []File{{StreamID: 10, Path: filepath.Join(massRoot, "missing.jpg")}}

	plan, err := Build(streams, files, opt)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Entries) != 0 || len(plan.Skipped) != 0 {
		t.Fatalf("plan = %+v, want an empty one", plan)
	}
}

func TestExecuteRemovesExactlyThePlanAndTheTokenTracksIt(t *testing.T) {
	vodRoot, massRoot, opt := setup(t)
	dir := filepath.Join(vodRoot, "10", "COMBINED")
	thumb := filepath.Join(massRoot, "a.jpg")
	write(t, filepath.Join(dir, "playlist.m3u8"), "x")
	write(t, filepath.Join(dir, "00000.ts"), "yy")
	write(t, thumb, "z")
	keep := filepath.Join(vodRoot, "11", "COMBINED", "playlist.m3u8")
	write(t, keep, "k")
	streams := []Stream{
		{ID: 10, DeletedAt: ago(2 * month), URLs: [3]string{vod("10")}},
		{ID: 11, URLs: [3]string{vod("11")}},
	}
	files := []File{{StreamID: 10, Path: thumb}}

	plan, err := Build(streams, files, opt)
	if err != nil {
		t.Fatal(err)
	}
	again, _ := Build(streams, files, opt)
	if plan.Token != again.Token {
		t.Fatal("the token is not deterministic")
	}

	res := Execute(plan)

	if len(res.Errors) != 0 || res.Files != 3 || res.Bytes != 4 {
		t.Fatalf("result = %+v, want 3 files, 4 bytes, no errors", res)
	}
	if exists(dir) || exists(filepath.Join(vodRoot, "10")) || exists(thumb) {
		t.Error("the planned files are still there")
	}
	if !exists(keep) {
		t.Error("a live stream's file was removed")
	}
	next, _ := Build(streams, files, opt)
	if len(next.Entries) != 0 {
		t.Errorf("a second plan lists %v", paths(next))
	}
}

// Files that appeared after the dry run were not reviewed, so they survive.
func TestExecuteLeavesFilesAddedAfterThePlan(t *testing.T) {
	vodRoot, _, opt := setup(t)
	dir := filepath.Join(vodRoot, "10", "COMBINED")
	write(t, filepath.Join(dir, "playlist.m3u8"), "x")
	plan, err := Build([]Stream{{ID: 10, DeletedAt: ago(2 * month), URLs: [3]string{vod("10")}}}, nil, opt)
	if err != nil {
		t.Fatal(err)
	}
	late := filepath.Join(dir, "late.ts")
	write(t, late, "new")

	res := Execute(plan)

	if !exists(late) {
		t.Error("an unreviewed file was deleted")
	}
	if len(res.Errors) == 0 {
		t.Error("the directory that could not be removed went unreported")
	}
}

// A live stream whose URL is still blank owns the directories named after its ID.
func TestALiveStreamProtectsItsOwnIDEvenWithoutAURL(t *testing.T) {
	vodRoot, _, opt := setup(t)
	write(t, filepath.Join(vodRoot, "9", "COMBINED", "playlist.m3u8"), "x")
	streams := []Stream{
		{ID: 9},
		{ID: 10, DeletedAt: ago(2 * month), URLs: [3]string{vod("9")}},
	}

	plan, err := Build(streams, nil, opt)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Entries) != 0 {
		t.Fatalf("entries = %v, want none", paths(plan))
	}
}

// With the VOD root inside the mass root, a File row for a segment is not listed twice.
func TestAFileInsideTheVODRootIsListedOnce(t *testing.T) {
	vodRoot, _, opt := setup(t)
	opt.MassRoot = filepath.Dir(vodRoot)
	seg := filepath.Join(vodRoot, "10", "COMBINED", "00000.ts")
	write(t, seg, "x")

	plan, err := Build(
		[]Stream{{ID: 10, DeletedAt: ago(2 * month), URLs: [3]string{vod("10")}}},
		[]File{{StreamID: 10, Path: seg}}, opt)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Files != 1 {
		t.Fatalf("files = %d, want 1", plan.Files)
	}
}
