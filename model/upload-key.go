package model

import "gorm.io/gorm"

type VideoType string

const (
	VideoTypeCombined     VideoType = "COMB"
	VideoTypePresentation           = "PRES"
	VideoTypeCamera                 = "CAM"
)

// UploadKey represents a key that is created when a user uploads a file,
// sent to the worker with the upload request and back to TUM-Live to authenticate the request.
type UploadKey struct {
	gorm.Model
	UploadKey string `gorm:"not null"`
	Stream    Stream
	StreamID  uint
	VideoType VideoType `gorm:"not null"`
}
