package model

type CameraPreset struct {
	Name          string `gorm:"not null;"`
	PresetID      int    `gorm:"primaryKey;autoIncrement:false"`
	Image         string
	LectureHallId uint `gorm:"primaryKey;autoIncrement:false"`
	Default       bool // this will be selected if there's no preference
}
