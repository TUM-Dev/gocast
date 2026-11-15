package dao

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/model"
)

//go:generate go tool mockgen -source=browser_recording.go -destination ../mock_dao/browser_recording.go

type BrowserRecordingDao interface {
	CreateBrowserRecording(ctx context.Context, rec *model.BrowserRecording) error
	GetBrowserRecording(ctx context.Context, id uint) (model.BrowserRecording, error)
	GetBrowserRecordingByTicket(ctx context.Context, ticketHash []byte) (model.BrowserRecording, error)
	// ListBrowserRecordings is a lecture's recordings that are not converted yet.
	ListBrowserRecordings(ctx context.Context, streamID uint) ([]model.BrowserRecording, error)
	// FinalizeBrowserRecording stops a recording from taking chunks. finalized says
	// whether this call did it; a second call changes nothing and answers false.
	FinalizeBrowserRecording(ctx context.Context, id uint, now time.Time) (finalized bool, err error)
	// ListPendingHandOffs is the finalized recordings no runner has, below the attempt limit.
	ListPendingHandOffs(ctx context.Context) ([]model.BrowserRecording, error)
	// ListUnfinalizedBrowserRecordings is the recordings created before `before` that still take chunks.
	ListUnfinalizedBrowserRecordings(ctx context.Context, before time.Time) ([]model.BrowserRecording, error)
	MarkBrowserRecordingHandedOff(ctx context.Context, id uint, now time.Time) error
	// SetBrowserRecordingImport names the hand-off about to start; see ImportID.
	SetBrowserRecordingImport(ctx context.Context, id uint, importID string) error
	// ListConvertingBrowserRecordings is the recordings a runner took and has not
	// reported on yet.
	ListConvertingBrowserRecordings(ctx context.Context) ([]model.BrowserRecording, error)
	// ConvertBrowserRecordingImport marks the recording of the import as converted.
	// Nothing for an import that is no longer the recording's current one.
	ConvertBrowserRecordingImport(ctx context.Context, importID string, now time.Time) error
	// FailBrowserRecordingImport takes back the hand-off of the import, to be handed off
	// again, as RequeueBrowserRecordingHandOff does. Nothing for an import that is no
	// longer the recording's current one.
	FailBrowserRecordingImport(ctx context.Context, importID, reason string) error
	// RequeueBrowserRecordingHandOff takes back a hand-off whose conversion was lost, to
	// be handed off again; it counts as a failed attempt.
	RequeueBrowserRecordingHandOff(ctx context.Context, id uint, reason string) error
	// MarkBrowserRecordingHandOffFailed records why a hand-off failed. counts says whether
	// it counts towards the attempt limit: a runner being away says nothing about the file.
	MarkBrowserRecordingHandOffFailed(ctx context.Context, id uint, reason string, counts bool) error
	// ResetBrowserRecordingHandOff lets a recording that hit the attempt limit be retried.
	ResetBrowserRecordingHandOff(ctx context.Context, id uint) error
	DeleteBrowserRecording(ctx context.Context, id uint) error
}

type browserRecordingDao struct {
	db *gorm.DB
}

func NewBrowserRecordingDao() BrowserRecordingDao { return browserRecordingDao{db: DB} }

func (d browserRecordingDao) CreateBrowserRecording(ctx context.Context, rec *model.BrowserRecording) error {
	return d.db.WithContext(ctx).Create(rec).Error
}

func (d browserRecordingDao) GetBrowserRecording(ctx context.Context, id uint) (rec model.BrowserRecording, err error) {
	err = d.db.WithContext(ctx).First(&rec, id).Error
	return rec, err
}

func (d browserRecordingDao) GetBrowserRecordingByTicket(ctx context.Context, ticketHash []byte) (rec model.BrowserRecording, err error) {
	err = d.db.WithContext(ctx).Where("ticket_hash = ?", ticketHash).First(&rec).Error
	return rec, err
}

func (d browserRecordingDao) ListBrowserRecordings(ctx context.Context, streamID uint) (recs []model.BrowserRecording, err error) {
	err = d.db.WithContext(ctx).Where("stream_id = ? AND converted_at IS NULL", streamID).Order("id").Find(&recs).Error
	return recs, err
}

func (d browserRecordingDao) FinalizeBrowserRecording(ctx context.Context, id uint, now time.Time) (bool, error) {
	res := d.db.WithContext(ctx).Model(&model.BrowserRecording{}).
		Where("id = ? AND finalized_at IS NULL", id).
		Update("finalized_at", now)
	return res.RowsAffected > 0, res.Error
}

func (d browserRecordingDao) ListPendingHandOffs(ctx context.Context) (recs []model.BrowserRecording, err error) {
	err = d.db.WithContext(ctx).
		Where("finalized_at IS NOT NULL AND handed_off_at IS NULL AND hand_off_attempts < ?", model.MaxBrowserRecordingHandOffAttempts).
		Order("id").Find(&recs).Error
	return recs, err
}

func (d browserRecordingDao) ListUnfinalizedBrowserRecordings(ctx context.Context, before time.Time) (recs []model.BrowserRecording, err error) {
	err = d.db.WithContext(ctx).Where("finalized_at IS NULL AND created_at < ?", before).Order("id").Find(&recs).Error
	return recs, err
}

func (d browserRecordingDao) MarkBrowserRecordingHandedOff(ctx context.Context, id uint, now time.Time) error {
	return d.db.WithContext(ctx).Model(&model.BrowserRecording{}).Where("id = ?", id).
		Updates(map[string]any{"handed_off_at": now, "hand_off_error": ""}).Error
}

func (d browserRecordingDao) ListConvertingBrowserRecordings(ctx context.Context) (recs []model.BrowserRecording, err error) {
	err = d.db.WithContext(ctx).Where("handed_off_at IS NOT NULL AND converted_at IS NULL").Order("id").Find(&recs).Error
	return recs, err
}

func (d browserRecordingDao) SetBrowserRecordingImport(ctx context.Context, id uint, importID string) error {
	return d.db.WithContext(ctx).Model(&model.BrowserRecording{}).Where("id = ?", id).Update("import_id", importID).Error
}

func (d browserRecordingDao) ConvertBrowserRecordingImport(ctx context.Context, importID string, now time.Time) error {
	return d.db.WithContext(ctx).Model(&model.BrowserRecording{}).
		Where("import_id = ? AND converted_at IS NULL", importID).
		Updates(map[string]any{"converted_at": now, "hand_off_error": ""}).Error
}

func (d browserRecordingDao) FailBrowserRecordingImport(ctx context.Context, importID, reason string) error {
	return d.requeue(ctx, d.db.WithContext(ctx).Where("import_id = ? AND converted_at IS NULL", importID), reason)
}

func (d browserRecordingDao) RequeueBrowserRecordingHandOff(ctx context.Context, id uint, reason string) error {
	return d.requeue(ctx, d.db.WithContext(ctx).Where("id = ?", id), reason)
}

// requeue takes back the hand-offs where selects. The import ID goes with it, so that a
// late report of the import taken back matches nothing.
func (d browserRecordingDao) requeue(_ context.Context, where *gorm.DB, reason string) error {
	if len(reason) > 512 {
		reason = reason[:512]
	}
	return where.Model(&model.BrowserRecording{}).Updates(map[string]any{
		"handed_off_at":     nil,
		"import_id":         "",
		"hand_off_attempts": gorm.Expr("hand_off_attempts + 1"),
		"hand_off_error":    reason,
	}).Error
}

func (d browserRecordingDao) MarkBrowserRecordingHandOffFailed(ctx context.Context, id uint, reason string, counts bool) error {
	if len(reason) > 512 {
		reason = reason[:512]
	}
	updates := map[string]any{"hand_off_error": reason}
	if counts {
		updates["hand_off_attempts"] = gorm.Expr("hand_off_attempts + 1")
	}
	return d.db.WithContext(ctx).Model(&model.BrowserRecording{}).Where("id = ?", id).Updates(updates).Error
}

func (d browserRecordingDao) ResetBrowserRecordingHandOff(ctx context.Context, id uint) error {
	return d.db.WithContext(ctx).Model(&model.BrowserRecording{}).Where("id = ?", id).
		Update("hand_off_attempts", 0).Error
}

func (d browserRecordingDao) DeleteBrowserRecording(ctx context.Context, id uint) error {
	return d.db.WithContext(ctx).Delete(&model.BrowserRecording{}, id).Error
}
