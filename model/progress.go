package model

// StreamProgress represents the progress of a stream or video. Currently, it is only used for VODs.
type StreamProgress struct {
	Progress float64
	// We need to use a primary key in order to use ON CONFLICT in dao/progress.go, same as e.g. https://www.sqlite.org/lang_conflict.html.
	StreamID uint `gorm:"primaryKey"`
	UserID   uint `gorm:"primaryKey"`
}
