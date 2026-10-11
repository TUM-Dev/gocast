package web

import (
	"html/template"
	"io"
	"testing"

	"github.com/Masterminds/sprig/v3"
)

// A pattern that matches no file makes every page that uses a template fail at
// request time, not at startup, so deleting a template directory has to take its
// pattern along. Rendering the error page also proves the partials it pulls in exist.
func TestTemplatesParseAndRenderTheErrorPage(t *testing.T) {
	tmpl, err := template.New("base").Funcs(sprig.FuncMap()).ParseFS(templateFS, templatePaths...)
	if err != nil {
		t.Fatalf("parsing %v: %v", templatePaths, err)
	}
	if err := tmpl.ExecuteTemplate(io.Discard, "error.gohtml", nil); err != nil {
		t.Fatalf("rendering error.gohtml: %v", err)
	}
}
