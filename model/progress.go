package model

type StreamProgress struct {
	Progress float64
	// We need to use a primary key in order to use ON CONFLICT in dao/progress.go, see https://www.sqlite.org/lang_conflict.html
	StreamID uint `gorm:"primaryKey"`
	UserID   uint `gorm:"primaryKey"`
}
