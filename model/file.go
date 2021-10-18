package model

import (
	"gorm.io/gorm"
	"strings"
)

type File struct {
	gorm.Model

	StreamID uint
	Path     string
}

func (f File) GetFriendlyFileName() string {
	pts := strings.Split(f.Path, "/")
	if len(pts) == 0 {
		return ""
	}
	return pts[len(pts)-1]
}
