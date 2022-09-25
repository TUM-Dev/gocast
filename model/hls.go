package model

import "gorm.io/gorm"

// Hls represents a single hls stream on disk defined by its root directory containing segments and a playlist.
type Hls struct {
	gorm.Model

	StreamVersion StreamVersion `gorm:"column:stream_version;type:text;not null;default:COMB"`
	StreamID      uint

	Path string `gorm:"column:path;type:text;not null"` // the path containing playlist.m3u8 and segment0000x.ts
}

// TableName returns the name of the table for the Hls model in the database.
func (*Hls) TableName() string {
	return "hls"
}
