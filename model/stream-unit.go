package model

import (
	"fmt"
	"github.com/microcosm-cc/bluemonday"
	"github.com/russross/blackfriday/v2"
	"gorm.io/gorm"
	"html/template"
)

type StreamUnit struct {
	gorm.Model

	UnitName        string `gorm:"not null"`
	UnitDescription string
	UnitStart       uint
	UnitEnd         uint
	StreamID        uint
}

func (s StreamUnit) GetUnitDurationMS() uint {
	return s.UnitEnd - s.UnitStart
}

func (s StreamUnit) GetRoundedUnitLen() string {
	lenS := (s.UnitEnd - s.UnitStart) / 1000
	lenM := lenS / 60
	lenH := lenM / 60
	lenM = lenM % 60
	lenS = lenS % 60
	if lenH > 0 {
		return fmt.Sprintf("%2dh, %2dmin", lenH, lenM)
	}
	return fmt.Sprintf("%2dmin, %2dsec", lenM, lenS)
}

func (s StreamUnit) GetDescriptionHTML() template.HTML {
	unsafe := blackfriday.Run([]byte(s.UnitDescription))
	html := bluemonday.
		UGCPolicy().
		AddTargetBlankToFullyQualifiedLinks(true).
		SanitizeBytes(unsafe)
	return template.HTML(html)
}
