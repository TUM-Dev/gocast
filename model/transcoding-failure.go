package model

import "gorm.io/gorm"

// TranscodingFailure represents a failed transcoding attempt
type TranscodingFailure struct {
	gorm.Model

	StreamID uint `gorm:"not null"`
	Stream   Stream
	Version  StreamVersion `gorm:"not null"`
	Logs     string        `gorm:"not null"`
	ExitCode int
	FilePath string `gorm:"not null"` // the source file that could not be transcoded
	Hostname string `gorm:"not null"` // the hostname of the worker that failed

	// Ignored by gorm:
	FriendlyTime string `gorm:"-"`
}

func (t *TranscodingFailure) AfterFind(tx *gorm.DB) (err error) {
	t.FriendlyTime = t.CreatedAt.Format("02.01.2006 15:04")
	return nil
}
