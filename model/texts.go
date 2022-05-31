package model

import (
	"github.com/microcosm-cc/bluemonday"
	"github.com/russross/blackfriday/v2"
	"gorm.io/gorm"
	"html/template"
)

const (
	TEXT_MARKDOWN = iota + 1
)

type Text struct {
	gorm.Model

	Name       string `gorm:"not null"` // e.g. 'privacy', 'imprint',...
	RawContent string `gorm:"text; not null"`
	Type       uint   `gorm:"not null; default: 1"`
}

func (mt Text) render() template.HTML {
	var renderedContent template.HTML = ""
	switch mt.Type {
	case TEXT_MARKDOWN:
		unsafe := blackfriday.Run([]byte(mt.RawContent))
		html := bluemonday.
			UGCPolicy().
			SanitizeBytes(unsafe)
		renderedContent = template.HTML(html)
	}
	return renderedContent
}
