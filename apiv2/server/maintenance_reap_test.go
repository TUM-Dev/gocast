package apiv2

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.uber.org/mock/gomock"
	"gorm.io/gorm"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
)

func reapAPI(t *testing.T, streams []dao.ReapStream, files []dao.ReapFile) (*API, string, *[]model.Audit) {
	t.Helper()
	root := t.TempDir()
	vod := filepath.Join(root, "vod")
	mass := filepath.Join(root, "mass")
	for _, d := range []string{vod, mass} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	reap := mock_dao.NewMockMediaReapDao(gomock.NewController(t))
	reap.EXPECT().ReapStreams().Return(streams, nil).AnyTimes()
	reap.EXPECT().ReapFiles().Return(files, nil).AnyTimes()
	audits, got := auditCapture(t)
	api := &API{
		dao:         dao.DaoWrapper{MediaReapDao: reap, AuditDao: audits},
		log:         slog.Default(),
		vodStorage:  vod,
		vodEdgeHost: "edge.live.rbg.tum.de",
		massStorage: mass,
	}
	return api, root, got
}

func deletedStream(id uint, url string) dao.ReapStream {
	return dao.ReapStream{
		ID:          id,
		CourseID:    1,
		DeletedAt:   gorm.DeletedAt{Time: time.Now().Add(-90 * 24 * time.Hour), Valid: true},
		PlaylistUrl: url,
	}
}

const reapURL = "https://edge.live.rbg.tum.de/vod/10/STREAM_VERSION_COMBINED/playlist.m3u8"

func writeReapFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestReapDeletedMediaIsADryRunByDefault(t *testing.T) {
	api, root, audits := reapAPI(t, []dao.ReapStream{deletedStream(10, reapURL)}, nil)
	seg := filepath.Join(root, "vod", "10", "STREAM_VERSION_COMBINED", "00000.ts")
	writeReapFile(t, seg)

	resp, err := api.ReapDeletedMedia(ctxAs(user(1, "A", "a@example.org", model.AdminType)), &protobuf.ReapDeletedMediaRequest{})
	if err != nil {
		t.Fatalf("ReapDeletedMedia: %v", err)
	}

	if resp.Executed || resp.PlanToken == "" || resp.TotalFiles != 1 || resp.TotalDirs != 1 {
		t.Fatalf("response = %+v, want a one-file plan that was not executed", resp)
	}
	if len(resp.Entries) != 2 || resp.Entries[1].Path != seg {
		t.Errorf("entries = %v, want the dir and its segment", resp.Entries)
	}
	if _, err := os.Stat(seg); err != nil {
		t.Error("a dry run removed a file")
	}
	if len(*audits) != 0 {
		t.Errorf("audits = %+v, want none for a read", *audits)
	}
}

func TestReapDeletedMediaPagesThroughTheDryRun(t *testing.T) {
	api, root, _ := reapAPI(t, []dao.ReapStream{deletedStream(10, reapURL)}, nil)
	writeReapFile(t, filepath.Join(root, "vod", "10", "STREAM_VERSION_COMBINED", "00000.ts"))
	writeReapFile(t, filepath.Join(root, "vod", "10", "STREAM_VERSION_COMBINED", "00001.ts"))
	ctx := ctxAs(user(1, "A", "a@example.org", model.AdminType))

	first, err := api.ReapDeletedMedia(ctx, &protobuf.ReapDeletedMediaRequest{PageSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	second, err := api.ReapDeletedMedia(ctx, &protobuf.ReapDeletedMediaRequest{PlanToken: first.PlanToken, Offset: 2, PageSize: 2})
	if err != nil {
		t.Fatal(err)
	}

	if len(first.Entries) != 2 || len(second.Entries) != 1 || first.TotalEntries != 3 {
		t.Errorf("pages = %d, %d of %d, want 2, 1 of 3", len(first.Entries), len(second.Entries), first.TotalEntries)
	}
	if _, err := api.ReapDeletedMedia(ctx, &protobuf.ReapDeletedMediaRequest{PlanToken: "stale", Offset: 2}); err == nil {
		t.Error("a page of an unknown dry run was served")
	}
}

func TestReapDeletedMediaExecuteNeedsTheDryRunsToken(t *testing.T) {
	api, root, audits := reapAPI(t, []dao.ReapStream{deletedStream(10, reapURL)}, nil)
	seg := filepath.Join(root, "vod", "10", "STREAM_VERSION_COMBINED", "00000.ts")
	writeReapFile(t, seg)
	ctx := ctxAs(user(1, "A", "a@example.org", model.AdminType))

	for name, token := range map[string]string{"no token": "", "wrong token": "deadbeef"} {
		if _, err := api.ReapDeletedMedia(ctx, &protobuf.ReapDeletedMediaRequest{Execute: true, PlanToken: token}); err == nil {
			t.Errorf("%s: executed", name)
		}
	}
	if _, err := os.Stat(seg); err != nil {
		t.Fatal("a refused execute removed a file")
	}
	if len(*audits) != 0 {
		t.Errorf("audits = %+v, want none for refusals", *audits)
	}

	dry, err := api.ReapDeletedMedia(ctx, &protobuf.ReapDeletedMediaRequest{})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := api.ReapDeletedMedia(ctx, &protobuf.ReapDeletedMediaRequest{Execute: true, PlanToken: dry.PlanToken})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	if !resp.Executed || resp.DeletedFiles != 1 || len(resp.Errors) != 0 {
		t.Errorf("response = %+v, want one file deleted", resp)
	}
	if _, err := os.Stat(filepath.Join(root, "vod", "10")); !os.IsNotExist(err) {
		t.Error("the stream's directory is still there")
	}
	if len(*audits) != 1 || (*audits)[0].Type != model.AuditMaintenance {
		t.Errorf("audits = %+v, want one maintenance entry", *audits)
	}
}

// A file that appears between the dry run and the execute changes the plan, so what
// was reviewed no longer describes what would be deleted.
func TestReapDeletedMediaRefusesWhenTheFilesChangedSinceTheDryRun(t *testing.T) {
	api, root, _ := reapAPI(t, []dao.ReapStream{deletedStream(10, reapURL)}, nil)
	writeReapFile(t, filepath.Join(root, "vod", "10", "STREAM_VERSION_COMBINED", "00000.ts"))
	ctx := ctxAs(user(1, "A", "a@example.org", model.AdminType))

	dry, err := api.ReapDeletedMedia(ctx, &protobuf.ReapDeletedMediaRequest{})
	if err != nil {
		t.Fatal(err)
	}
	late := filepath.Join(root, "vod", "10", "STREAM_VERSION_COMBINED", "00001.ts")
	writeReapFile(t, late)

	if _, err := api.ReapDeletedMedia(ctx, &protobuf.ReapDeletedMediaRequest{Execute: true, PlanToken: dry.PlanToken}); err == nil {
		t.Fatal("executed a plan that changed")
	}
	if _, err := os.Stat(late); err != nil {
		t.Error("a file was removed")
	}
}

func TestReapDeletedMediaWithoutStorageAnswers503(t *testing.T) {
	api := &API{log: slog.Default()}
	if _, err := api.ReapDeletedMedia(ctxAs(user(1, "A", "a@example.org", model.AdminType)), &protobuf.ReapDeletedMediaRequest{}); err == nil {
		t.Fatal("answered without any storage configured")
	}
}

// A --test-env server holds a copy of the database against prod's storage; it may list
// what would go but must never delete.
func TestReapDeletedMediaNeverExecutesInTestEnv(t *testing.T) {
	api, root, audits := reapAPI(t, []dao.ReapStream{deletedStream(10, reapURL)}, nil)
	seg := filepath.Join(root, "vod", "10", "STREAM_VERSION_COMBINED", "00000.ts")
	writeReapFile(t, seg)
	ctx := ctxAs(user(1, "A", "a@example.org", model.AdminType))

	dry, err := api.ReapDeletedMedia(ctx, &protobuf.ReapDeletedMediaRequest{})
	if err != nil {
		t.Fatal(err)
	}
	prev := tools.TestEnv
	tools.TestEnv = tools.TestEnvPolicy{Enabled: true}
	t.Cleanup(func() { tools.TestEnv = prev })

	if _, err := api.ReapDeletedMedia(ctx, &protobuf.ReapDeletedMediaRequest{}); err != nil {
		t.Errorf("a dry run was refused in --test-env: %v", err)
	}
	if _, err := api.ReapDeletedMedia(ctx, &protobuf.ReapDeletedMediaRequest{Execute: true, PlanToken: dry.PlanToken}); err == nil {
		t.Fatal("executed in --test-env")
	}
	if _, err := os.Stat(seg); err != nil {
		t.Error("a file was removed in --test-env")
	}
	if len(*audits) != 0 {
		t.Errorf("audits = %+v, want none", *audits)
	}
}

// Without the edge host every URL would be skipped as foreign and the dry run would read
// as "nothing to delete"; that has to be an error instead.
func TestReapDeletedMediaNeedsToKnowTheEdgeHost(t *testing.T) {
	api, _, _ := reapAPI(t, []dao.ReapStream{deletedStream(10, reapURL)}, nil)
	api.vodEdgeHost = ""
	prev := tools.Cfg.VodURLTemplate
	tools.Cfg.VodURLTemplate = ""
	t.Cleanup(func() { tools.Cfg.VodURLTemplate = prev })

	if _, err := api.ReapDeletedMedia(ctxAs(user(1, "A", "a@example.org", model.AdminType)), &protobuf.ReapDeletedMediaRequest{}); err == nil {
		t.Fatal("answered without knowing which host serves /vod/")
	}
}
