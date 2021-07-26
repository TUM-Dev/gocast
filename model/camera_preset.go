package model

type CameraPreset struct {
	Name          string
	PresetID      int    `gorm:"primaryKey;autoIncrement:false"`
	Image         string
	LectureHallId uint   `gorm:"primaryKey;autoIncrement:false"`
}
