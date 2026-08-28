package model

// StreamProgress represents the progress of a stream or video. Currently, it is only used for VoDs.
type StreamProgress struct {
	Progress float64 `gorm:"not null" json:"progress"`              // The progress of the stream as represented as a floating point value between 0 and 1.
	Watched  bool    `gorm:"not null;default:false" json:"watched"` // Whether the user has marked the stream as watched.

	// We need to use a primary key in order to use ON CONFLICT in dao/progress.go, same as e.g. https://www.sqlite.org/lang_conflict.html.
	StreamID uint `gorm:"primaryKey" json:"streamId"`
	// The primary key is (stream_id, user_id), so lookups by user alone cannot use it.
	// GetProgressesForUser/LoadProgress filter on user_id, hence the extra index.
	UserID uint `gorm:"primaryKey;index" json:"-"`
}
