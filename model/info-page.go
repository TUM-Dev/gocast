package model

import (
	"html/template"

	"github.com/microcosm-cc/bluemonday"
	"github.com/russross/blackfriday/v2"
	"gorm.io/gorm"
)

type InfoPageType uint

const (
	INFOPAGE_MARKDOWN InfoPageType = iota + 1
)

type InfoPage struct {
	gorm.Model

	Name       string       `gorm:"not null"` // e.g. 'privacy', 'imprint',...
	RawContent string       `gorm:"text; not null"`
	Type       InfoPageType `gorm:"not null; default: 1"`
}

func (mt *InfoPage) Render() template.HTML {
	var renderedContent template.HTML = ""
	switch mt.Type {
	case INFOPAGE_MARKDOWN:
		unsafe := blackfriday.Run([]byte(mt.RawContent))
		html := bluemonday.
			UGCPolicy().
			SanitizeBytes(unsafe)
		renderedContent = template.HTML(html)
	default:
		renderedContent = template.HTML(mt.RawContent)
	}
	return renderedContent
}
