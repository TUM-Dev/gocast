package apiv2

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/TUM-Dev/gocast/tools/mediareap"
)

// reapGracePeriod is how long a stream stays restorable before its files may go.
// UnDeleteCourse restores a course's streams, so the files must outlive that window.
const reapGracePeriod = 30 * 24 * time.Hour

const (
	defaultReapPageSize = 500
	maxReapPageSize     = 5000
	// reapPlanTTL is how long a dry run's plan serves further pages. Walking the
	// directories is the expensive part, and paging recomputed it each time would be
	// slow on a plan with hundreds of thousands of segments.
	reapPlanTTL = 15 * time.Minute
)

// WithVODStorage sets where the runner's <streamID>/STREAM_VERSION_<X>/ directories are mounted
// into this server. Optional: without it ReapDeletedMedia answers 503, since reaping
// needs to see the files it lists.
func WithVODStorage(dir string) Option {
	return func(a *API) { a.vodStorage = dir }
}

// reapCache keeps the plan of the latest dry run for paging through it.
type reapCache struct {
	// executing admits one execute at a time; a second would race the first's removals.
	executing sync.Mutex

	mu   sync.Mutex
	plan *mediareap.Plan
	at   time.Time
}

func (c *reapCache) put(p *mediareap.Plan) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.plan, c.at = p, time.Now()
}

func (c *reapCache) get(token string) *mediareap.Plan {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.plan == nil || c.plan.Token != token || time.Since(c.at) > reapPlanTTL {
		return nil
	}
	return c.plan
}

// ReapDeletedMedia deletes the files of soft-deleted streams, which soft-deleting
// never does.
//
// Without execute it is a dry run listing every file and directory an execute would
// remove. Executing needs the token of the dry run the admin reviewed and recomputes
// the plan: if anything changed since (a stream restored, a copy made, a file added),
// the tokens differ and nothing is deleted, so only what was reviewed is ever removed.
func (a *API) ReapDeletedMedia(ctx context.Context, req *protobuf.ReapDeletedMediaRequest) (*protobuf.ReapDeletedMediaResponse, error) {
	if a.vodStorage == "" && a.massStorage == "" {
		return nil, e.WithStatus(http.StatusServiceUnavailable, errors.New("no media storage is configured"))
	}

	if !req.GetExecute() {
		return a.reapDryRun(req)
	}

	// --test-env serves a copy of the production database against prod's storage. The
	// copy ages, so its idea of what is deleted and what is still referenced can be
	// wrong, and a delete there would hit files production uses. Listing stays allowed.
	if tools.TestEnv.Enabled {
		return nil, e.WithStatus(http.StatusForbidden, errors.New("deleting media is disabled in --test-env"))
	}
	if req.GetPlanToken() == "" {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("run a dry run first and pass its plan_token"))
	}
	if !a.reapCache.executing.TryLock() {
		return nil, e.WithStatus(http.StatusConflict, errors.New("a reap is already running"))
	}
	defer a.reapCache.executing.Unlock()
	plan, err := a.reapPlan()
	if err != nil {
		return nil, err
	}
	if plan.Token != req.GetPlanToken() {
		return nil, e.WithStatus(http.StatusPreconditionFailed, errors.New("the files changed since the dry run; review it again"))
	}

	res := mediareap.Execute(plan)
	a.audit(ctx, model.AuditMaintenance, fmt.Sprintf("reaped media of deleted streams: %d files, %d bytes, %d errors", res.Files, res.Bytes, len(res.Errors)))
	a.reapCache.put(nil)
	return &protobuf.ReapDeletedMediaResponse{
		Executed:     true,
		PlanToken:    plan.Token,
		DeletedFiles: uint32(res.Files),
		DeletedBytes: uint64(res.Bytes),
		Errors:       res.Errors,
	}, nil
}

func (a *API) reapDryRun(req *protobuf.ReapDeletedMediaRequest) (*protobuf.ReapDeletedMediaResponse, error) {
	first := req.GetOffset() == 0
	var plan *mediareap.Plan
	if !first {
		plan = a.reapCache.get(req.GetPlanToken())
		if plan == nil {
			return nil, e.WithStatus(http.StatusPreconditionFailed, errors.New("the dry run expired; run it again"))
		}
	} else {
		var err error
		if plan, err = a.reapPlan(); err != nil {
			return nil, err
		}
		a.reapCache.put(plan)
	}

	size := int(req.GetPageSize())
	if size == 0 {
		size = defaultReapPageSize
	}
	size = min(size, maxReapPageSize)
	start := min(int(req.GetOffset()), len(plan.Entries))
	end := min(start+size, len(plan.Entries))

	resp := &protobuf.ReapDeletedMediaResponse{
		TotalEntries: uint32(len(plan.Entries)),
		TotalDirs:    uint32(plan.Dirs),
		TotalFiles:   uint32(plan.Files),
		TotalBytes:   uint64(plan.Bytes),
		PlanToken:    plan.Token,
	}
	for _, en := range plan.Entries[start:end] {
		resp.Entries = append(resp.Entries, reapEntryMessage(en))
	}
	if first {
		for _, s := range plan.Skipped {
			resp.Skipped = append(resp.Skipped, &protobuf.ReapSkipped{Path: s.Path, StreamIds: uints(s.StreamIDs), Reason: s.Reason})
		}
	}
	return resp, nil
}

// reapPlan builds the plan from the current database and disk.
func (a *API) reapPlan() (*mediareap.Plan, error) {
	streams, err := a.dao.MediaReapDao.ReapStreams()
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	files, err := a.dao.MediaReapDao.ReapFiles()
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	in := make([]mediareap.Stream, 0, len(streams))
	for _, s := range streams {
		in = append(in, mediareap.Stream{
			ID:        s.ID,
			CourseID:  s.CourseID,
			DeletedAt: s.Deleted(),
			URLs:      [3]string{s.PlaylistUrl, s.PlaylistUrlPRES, s.PlaylistUrlCAM},
		})
	}
	inFiles := make([]mediareap.File, 0, len(files))
	for _, f := range files {
		inFiles = append(inFiles, mediareap.File{StreamID: f.StreamID, Path: f.Path, Deleted: f.DeletedAt.Valid})
	}

	plan, err := mediareap.Build(in, inFiles, mediareap.Options{
		VODRoot:  a.vodStorage,
		MassRoot: a.massStorage,
		Cutoff:   time.Now().Add(-reapGracePeriod),
	})
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	return plan, nil
}

func reapEntryMessage(en mediareap.Entry) *protobuf.ReapEntry {
	kind := protobuf.ReapEntryKind_REAP_ENTRY_KIND_FILE
	switch en.Kind {
	case mediareap.KindVODDir:
		kind = protobuf.ReapEntryKind_REAP_ENTRY_KIND_VOD_DIR
	case mediareap.KindVODFile:
		kind = protobuf.ReapEntryKind_REAP_ENTRY_KIND_VOD_FILE
	}
	return &protobuf.ReapEntry{
		Kind:      kind,
		Path:      en.Path,
		Bytes:     uint64(en.Bytes),
		StreamIds: uints(en.StreamIDs),
		CourseIds: uints(en.CourseIDs),
		DeletedAt: timestamppb.New(en.DeletedAt),
	}
}

func uints(ids []uint) []uint32 {
	out := make([]uint32, 0, len(ids))
	for _, id := range ids {
		out = append(out, uint32(id))
	}
	return out
}
