package model

type CameraPreset struct {
	Name          string // eg. Krusche Pult
	PresetID      int    `gorm:"primaryKey;autoIncrement:false"` // eg. 2
	Image         string // currently unused
	LectureHallId uint   `gorm:"primaryKey;autoIncrement:false"`
}
