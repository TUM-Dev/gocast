package model

type StreamProgress struct {
	Progress float64
	StreamID uint `gorm:"primaryKey"`
	UserID uint `gorm:"primaryKey"`
}