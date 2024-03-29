package tools

import (
	"html/template"
	"io"

	"github.com/Masterminds/sprig/v3"
)

type TemplateExecutor interface {
	ExecuteTemplate(w io.Writer, name string, data interface{}) error
}

type DebugTemplateExecutor struct {
	Patterns []string
}

func (e DebugTemplateExecutor) ExecuteTemplate(w io.Writer, name string, data interface{}) error {
	if len(e.Patterns) == 0 {
		panic("Provide at least one pattern for the debug template executor.")
	}

	t, err := template.New("base").Funcs(sprig.FuncMap()).ParseGlob(e.Patterns[0])
	if err != nil {
		logger.Error("Failed to load pattern: '"+e.Patterns[0], "err", err.Error())
	}

	for i := 1; i < len(e.Patterns); i++ {
		pattern := e.Patterns[i]
		_, err := t.ParseGlob(pattern)
		if err != nil {
			logger.Error("Failed to load pattern: '"+pattern+"'.", "err", err.Error())
		}
	}

	return t.ExecuteTemplate(w, name, data)
}

type ReleaseTemplateExecutor struct {
	Template *template.Template
}

func (e ReleaseTemplateExecutor) ExecuteTemplate(w io.Writer, name string, data interface{}) error {
	return e.Template.ExecuteTemplate(w, name, data)
}
