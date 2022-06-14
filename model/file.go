package model

import (
	"gorm.io/gorm"
	"net/url"
	"strings"
)

const (
	FILETYPE_DOWNLOAD = iota + 1
	FILETYPE_ATTACHMENT
	FILETYPE_IMAGE_JPG
)

type File struct {
	gorm.Model

	StreamID uint   `gorm:"not null"`
	Path     string `gorm:"not null"`
	Filename string
	Type     uint `gorm:"not null; default: 1"`
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

	if f.Filename != "" {
		return f.Filename
	}
	return "Default view"
}

func (f File) IsURL() bool {
	parsedUrl, err := url.Parse(f.Path)
	if err != nil {
		return false
	}
	return parsedUrl.Scheme == "https://" || parsedUrl.Scheme == "http://"
}
