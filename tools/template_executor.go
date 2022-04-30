package tools

import (
	"html/template"
	"io"
	"io/fs"
)

type TemplateExecutor interface {
	ExecuteTemplate(w io.Writer, name string, data interface{}) error
}

type DebugTemplateExecutor struct {
	Fs       fs.FS
	Patterns []string
}

func (e DebugTemplateExecutor) ExecuteTemplate(w io.Writer, name string, data interface{}) error {
	t := template.Must(template.ParseFS(e.Fs, e.Patterns...))
	return t.ExecuteTemplate(w, name, data)
}

type ReleaseTemplateExecutor struct {
	Template *template.Template
}

func (e ReleaseTemplateExecutor) ExecuteTemplate(w io.Writer, name string, data interface{}) error {
	return e.Template.ExecuteTemplate(w, name, data)
}
