package api

import (
	"bytes"
	"html/template"
	"strings"
	"testing"
	"time"

	"github.com/TUM-Dev/gocast/model"
)

func TestMailCourseRegisteredTemplate(t *testing.T) {
	start := time.Date(2026, 1, 12, 14, 0, 0, 0, time.UTC)
	end := start.Add(90 * time.Minute)
	course := model.Course{
		Name:  "Testkurs",
		Token: "abc123",
		Streams: []model.Stream{
			{Start: start, End: end, RoomName: "MW 0001"},
		},
	}
	admin := &model.User{Name: "Max Mustermann"}

	cases := []struct {
		name         string
		optIn        bool
		wantContains []string
		wantAbsent   []string
	}{
		{
			name:  "opt-in course must be activated by the admin",
			optIn: true,
			wantContains: []string{
				"müssen Sie diese hier aktivieren",
				"https://live.rbg.tum.de/edit-course/?token=abc123",
				"you must activate it here",
			},
			wantAbsent: []string{
				"edit-course/opt-out",
			},
		},
		{
			name:  "default-active course offers an opt-out link instead",
			optIn: false,
			wantContains: []string{
				"können Sie Ihre Veranstaltung hier entfernen",
				"https://live.rbg.tum.de/edit-course/opt-out?token=abc123",
				"you can remove your course here",
			},
			wantAbsent: []string{
				"https://live.rbg.tum.de/edit-course/?token=abc123",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			templ, err := template.ParseFS(staticFS, "template/*.gotemplate")
			if err != nil {
				t.Fatalf("parse template: %v", err)
			}

			var body bytes.Buffer
			err = templ.ExecuteTemplate(&body, "mail-course-registered.gotemplate", MailTmpl{
				Name:   "Max Mustermann",
				OptIn:  tc.optIn,
				Course: course,
				Users:  []*model.User{admin},
			})
			if err != nil {
				t.Fatalf("execute template: %v", err)
			}
			rendered := body.String()

			for _, want := range tc.wantContains {
				if !strings.Contains(rendered, want) {
					t.Errorf("rendered email missing %q\n--- rendered ---\n%s", want, rendered)
				}
			}
			for _, notWant := range tc.wantAbsent {
				if strings.Contains(rendered, notWant) {
					t.Errorf("rendered email unexpectedly contains %q", notWant)
				}
			}
			if !strings.Contains(rendered, "Max Mustermann, ") {
				t.Errorf("rendered email missing associated admin listing")
			}
			if !strings.Contains(rendered, "MW 0001") {
				t.Errorf("rendered email missing lecture room listing")
			}
		})
	}
}
