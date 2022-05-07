package model

import (
	"gorm.io/gorm"
	"strings"
)

const (
	FILETYPE_DOWNLOAD   = 0
	FILETYPE_ATTACHMENT = 1
)

type File struct {
	gorm.Model

	StreamID uint   `gorm:"not null"`
	Path     string `gorm:"not null"`
	Filename string `gorm:"null;"`
	Type     uint   `gorm:"default: 0"`
}

func (f File) GetDownloadFileName() string {
	pts := strings.Split(f.Path, "/")
	if len(pts) == 0 {
		return ""
	}
	return pts[len(pts)-1]
}

func (f File) GetFriendlyFileName() string {
	fn := f.GetDownloadFileName()
	if strings.Contains(strings.ToLower(fn), "cam") {
		return "Camera-view"
	}
	if strings.Contains(strings.ToLower(fn), "pres") {
		return "Presentation"
	}
	return "Default view"
}

func (f File) IsAbsolutePath() bool {
	return strings.HasPrefix(f.Path, "https://") || strings.HasPrefix(f.Path, "http://")
}
