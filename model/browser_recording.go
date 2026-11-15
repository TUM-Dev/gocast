package model

import "time"

// BrowserRecording is one version of a lecture recorded in an administrator's browser.
// The browser uploads it in chunks while it records, which the server keeps on disk,
// not here; see the apiv2 recording store. Once finalized, the chunks are handed to a
// runner as an upload of that version. They stay until the runner has made the VOD,
// as the runner keeps nothing of an import it loses, and the row stays to tell so.
//
// A recording has segments: each is what one MediaRecorder made, a file of its own. A
// new one starts when the recording resumes after its tracks ended or the tab crashed.
type BrowserRecording struct {
	ID          uint          `gorm:"primaryKey"`
	StreamID    uint          `gorm:"not null;index"`
	Version     StreamVersion `gorm:"type:varchar(4);not null"`
	MimeType    string        `gorm:"type:varchar(128);not null"`
	CreatedByID uint          `gorm:"not null"`
	// TicketHash is the SHA-256 of the ticket the chunks are uploaded with. It is not
	// bound to a login, so a session that expires during a lecture does not stop uploads.
	TicketHash []byte `gorm:"type:binary(32);not null;uniqueIndex"`
	// FinalizedAt is set once the recording takes no more chunks.
	FinalizedAt *time.Time
	// ImportID names the current hand-off to the runner, which reports the conversion
	// with it, so that a report can't be taken for another import of the same version.
	// A new one with every hand-off: a report of an earlier one no longer matches.
	ImportID string `gorm:"type:varchar(64);not null;default:'';index"`
	// HandedOffAt is set once a runner has taken the recording to convert.
	HandedOffAt *time.Time
	// ConvertedAt is set once the VOD is there; its chunks are gone then.
	ConvertedAt *time.Time
	// HandOffAttempts counts the hand-offs that failed. Retrying stops at
	// MaxBrowserRecordingHandOffAttempts until an administrator retries.
	HandOffAttempts int    `gorm:"not null;default:0"`
	HandOffError    string `gorm:"type:varchar(512);not null;default:''"`
	CreatedAt       time.Time
}

// MaxBrowserRecordingHandOffAttempts is how often a finalized recording is handed off
// on its own before it waits for an administrator.
const MaxBrowserRecordingHandOffAttempts = 5
