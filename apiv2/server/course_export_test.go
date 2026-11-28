package apiv2

import (
	"strings"
	"testing"
	"time"

	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"gorm.io/gorm"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/model"
)

func TestExportCourseVideos(t *testing.T) {
	useJWTKey(t)
	day := time.Date(2025, 10, 14, 10, 0, 0, 0, time.UTC)
	streams := []model.Stream{
		{Model: gorm.Model{ID: 1}, Recording: true, Start: day, PlaylistUrl: "https://edge/vod/a/comb.m3u8", PlaylistUrlCAM: "https://edge/vod/a/cam.m3u8"},
		// A second unnamed lecture on the same day must not overwrite the first.
		{Model: gorm.Model{ID: 2}, Recording: true, Start: day, PlaylistUrl: "https://edge/vod/b/comb.m3u8"},
		// A lecture without the requested version (unrecorded ones are filtered by the query).
		{Model: gorm.Model{ID: 4}, Recording: true, Start: day, PlaylistUrlPRES: "https://edge/vod/d/pres.m3u8"},
		// The signer leaves lrz.de playlists unsigned, which the edge would refuse.
		{Model: gorm.Model{ID: 5}, Recording: true, Start: day, PlaylistUrl: "https://lrz.de/vod/e/comb.m3u8"},
	}
	api, m := courseAdminAPI(t, adminCourse)
	m.streams.EXPECT().GetRecordedStreamsForExport(gomock.Any(), adminCourse.ID).Return(streams, nil)
	m.audits.EXPECT().Create(gomock.Any()).Return(nil)

	res, err := api.ExportCourseVideos(asCaller(courseLecturer), &protobuf.ExportCourseVideosRequest{CourseId: 5, Kind: protobuf.WatchViewKind_WATCH_VIEW_KIND_COMBINED})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Videos) != 2 {
		t.Fatalf("got %d videos, want 2: %v", len(res.Videos), res.Videos)
	}
	first, second := res.Videos[0], res.Videos[1]
	if first.FileName != "brau_2025-10-14_COMB.mp4" || second.FileName != "brau_2025-10-14_COMB_2.mp4" {
		t.Errorf("file names %q, %q", first.FileName, second.FileName)
	}
	for _, v := range res.Videos {
		if !strings.Contains(v.Url, "?jwt=") || !strings.HasSuffix(v.Url, "&download=1") {
			t.Errorf("url %q is not a signed download link", v.Url)
		}
	}
	if !strings.HasPrefix(first.Url, "https://edge/vod/a/comb.m3u8") {
		t.Errorf("first url %q is not the combined playlist", first.Url)
	}
}

// Without a kind, every version a lecture has is listed, each under its own file name.
func TestExportCourseVideosAllVersions(t *testing.T) {
	useJWTKey(t)
	day := time.Date(2025, 10, 14, 10, 0, 0, 0, time.UTC)
	streams := []model.Stream{
		{Model: gorm.Model{ID: 1}, Recording: true, Start: day,
			PlaylistUrl: "https://edge/vod/a/comb.m3u8", PlaylistUrlPRES: "https://edge/vod/a/pres.m3u8", PlaylistUrlCAM: "https://edge/vod/a/cam.m3u8"},
		{Model: gorm.Model{ID: 2}, Recording: true, Start: day, PlaylistUrl: "https://edge/vod/b/comb.m3u8"},
		// Only the part the signer leaves unsigned: nothing to download.
		{Model: gorm.Model{ID: 3}, Recording: true, Start: day, PlaylistUrl: "https://lrz.de/vod/c/comb.m3u8"},
	}
	api, m := courseAdminAPI(t, adminCourse)
	m.streams.EXPECT().GetRecordedStreamsForExport(gomock.Any(), adminCourse.ID).Return(streams, nil)
	m.audits.EXPECT().Create(gomock.Any()).Return(nil)

	res, err := api.ExportCourseVideos(asCaller(courseLecturer), &protobuf.ExportCourseVideosRequest{CourseId: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Videos) != 4 {
		t.Fatalf("got %d videos, want 4: %v", len(res.Videos), res.Videos)
	}
	seen := map[string]bool{}
	for _, v := range res.Videos {
		if seen[v.FileName] {
			t.Errorf("file name %q is used twice", v.FileName)
		}
		seen[v.FileName] = true
	}
	for i, suffix := range []string{"_COMB.mp4", "_PRES.mp4", "_CAM.mp4"} {
		if !strings.HasSuffix(res.Videos[i].FileName, suffix) || res.Videos[i].StreamId != 1 {
			t.Errorf("video %d = %v, want stream 1 %s", i, res.Videos[i], suffix)
		}
	}
}

func TestExportCourseVideosKind(t *testing.T) {
	useJWTKey(t)
	streams := []model.Stream{
		{Model: gorm.Model{ID: 1}, Recording: true, PlaylistUrl: "https://edge/vod/a/comb.m3u8", PlaylistUrlCAM: "https://edge/vod/a/cam.m3u8"},
		{Model: gorm.Model{ID: 2}, Recording: true, PlaylistUrl: "https://edge/vod/b/comb.m3u8"},
	}
	api, m := courseAdminAPI(t, adminCourse)
	m.streams.EXPECT().GetRecordedStreamsForExport(gomock.Any(), adminCourse.ID).Return(streams, nil)
	m.audits.EXPECT().Create(gomock.Any()).Return(nil)

	res, err := api.ExportCourseVideos(asCaller(courseLecturer), &protobuf.ExportCourseVideosRequest{
		CourseId: 5, Kind: protobuf.WatchViewKind_WATCH_VIEW_KIND_CAMERA,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Videos) != 1 || !strings.HasPrefix(res.Videos[0].Url, "https://edge/vod/a/cam.m3u8") {
		t.Errorf("got %v, want only stream 1's camera version", res.Videos)
	}
	if !strings.HasSuffix(res.Videos[0].FileName, "_CAM.mp4") {
		t.Errorf("file name %q does not name the version", res.Videos[0].FileName)
	}
}

func TestExportCourseVideosRefusals(t *testing.T) {
	api, _ := courseAdminAPI(t, adminCourse)
	ctx := asCaller(courseLecturer)

	_, err := api.ExportCourseVideos(ctx, &protobuf.ExportCourseVideosRequest{CourseId: 0})
	wantCode(t, err, codes.NotFound)
	_, err = api.ExportCourseVideos(ctx, &protobuf.ExportCourseVideosRequest{CourseId: 5, Kind: 99})
	wantCode(t, err, codes.InvalidArgument)
}

func TestUniqueExportName(t *testing.T) {
	used := map[string]bool{}
	got := []string{
		uniqueExportName(used, "x.mp4", 1),
		uniqueExportName(used, "x.mp4", 7),
		// Already spelled like the suffixed name above.
		uniqueExportName(used, "x_7.mp4", 8),
		uniqueExportName(used, "", 9),
	}
	want := []string{"x.mp4", "x_7.mp4", "x_7_8.mp4", "9.mp4"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("name %d = %q, want %q", i, got[i], want[i])
		}
	}
}
