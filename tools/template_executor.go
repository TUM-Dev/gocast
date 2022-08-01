package tools

import (
	"context"
	"github.com/Masterminds/sprig/v3"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"html/template"
	"io"
)

type TemplateExecutor interface {
	ExecuteTemplate(ctx context.Context, w io.Writer, name string, data interface{}) error
}

type DebugTemplateExecutor struct {
	Patterns []string
}

func (e DebugTemplateExecutor) ExecuteTemplate(c context.Context, w io.Writer, name string, data interface{}) error {
	var span *sentry.Span
	if ctx, ok := c.(*gin.Context); ok {
		if s, ok := ctx.Get("sentry.span"); ok {
			if sTmp, ok := s.(*sentry.Span); ok {
				span = sentry.StartSpan(sTmp.Context(), "HTML Render")
			}
		}
	}
	if len(e.Patterns) == 0 {
		panic("Provide at least one pattern for the debug template executor.")
	}

	var t, err = template.New("base").Funcs(sprig.FuncMap()).ParseGlob(e.Patterns[0])
	if err != nil {
		log.Print("Failed to load pattern: '" + e.Patterns[0] + "'. Error: " + err.Error())
	}

	for i := 1; i < len(e.Patterns); i++ {
		pattern := e.Patterns[i]
		_, err := t.ParseGlob(pattern)
		if err != nil {
			log.Print("Failed to load pattern: '" + pattern + "'. Error: " + err.Error())
		}
	}

	err = t.ExecuteTemplate(w, name, data)
	if span != nil {
		span.Finish()
	}
	return err
}

type ReleaseTemplateExecutor struct {
	Template *template.Template
}

func (e ReleaseTemplateExecutor) ExecuteTemplate(c context.Context, w io.Writer, name string, data interface{}) error {
	var span *sentry.Span
	if ctx, ok := c.(*gin.Context); ok {
		if s, ok := ctx.Get("sentry.span"); ok {
			if sTmp, ok := s.(*sentry.Span); ok {
				span = sentry.StartSpan(sTmp.Context(), "HTML Render")
			}
		}
	}
	err := e.Template.ExecuteTemplate(w, name, data)
	if span != nil {
		span.Finish()
	}
	return err
}
