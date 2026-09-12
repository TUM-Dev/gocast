package tools

import "testing"

// CanonicalURL feeds <link rel="canonical"> and the share links, so every method is
// pinned against the two shapes the deployment config actually takes: a bare origin and
// an origin with a path prefix, each with and without a trailing slash.
func TestCanonicalURL(t *testing.T) {
	tests := []struct {
		name string
		base string
		got  func(CanonicalURL) string
		want string
	}{
		{
			// The scheme survives joining: path.Join used to collapse the "//" here and
			// ship a "https:/..." canonical tag.
			name: "a bare origin keeps its scheme once something is joined",
			base: "https://live.rbg.tum.de",
			got:  func(c CanonicalURL) string { return c.Login() },
			want: "https://live.rbg.tum.de/login",
		},
		{
			name: "Root hands back the configured base untouched",
			base: "https://live.rbg.tum.de",
			got:  CanonicalURL.Root,
			want: "https://live.rbg.tum.de",
		},
		{
			name: "Root keeps a trailing slash, unlike every other method",
			base: "https://live.rbg.tum.de/",
			got:  CanonicalURL.Root,
			want: "https://live.rbg.tum.de/",
		},
		{
			name: "a trailing slash on the base does not double up",
			base: "https://live.rbg.tum.de/",
			got:  func(c CanonicalURL) string { return c.Login() },
			want: "https://live.rbg.tum.de/login",
		},
		{
			name: "a base with a path prefix keeps the prefix",
			base: "https://example.org/tum",
			got:  func(c CanonicalURL) string { return c.Login() },
			want: "https://example.org/tum/login",
		},
		{
			name: "a base with a path prefix and a trailing slash keeps the prefix once",
			base: "https://example.org/tum/",
			got:  func(c CanonicalURL) string { return c.Login() },
			want: "https://example.org/tum/login",
		},
		{
			// An unset canonicalURL in the config cannot produce an absolute URL; a
			// root-relative path at least resolves from any page it is rendered on.
			name: "an empty base produces a root-relative path",
			base: "",
			got:  func(c CanonicalURL) string { return c.Login() },
			want: "/login",
		},
		{
			// The methods are called from templates and cannot report an error, so an
			// unparsable base degrades the same way an empty one does.
			name: "an unparsable base degrades to a root-relative path",
			base: "://not-a-url",
			got:  func(c CanonicalURL) string { return c.Login() },
			want: "/login",
		},
		{
			name: "Course joins year, term and slug",
			base: "https://live.rbg.tum.de",
			got:  func(c CanonicalURL) string { return c.Course(2023, "W", "eidi") },
			want: "https://live.rbg.tum.de/course/2023/W/eidi",
		},
		{
			name: "Course renders a negative year rather than rejecting it",
			base: "https://live.rbg.tum.de",
			got:  func(c CanonicalURL) string { return c.Course(-1, "S", "eidi") },
			want: "https://live.rbg.tum.de/course/-1/S/eidi",
		},
		{
			// Slugs come from user-editable course settings, so a separator in one is
			// escaped into the segment instead of opening a new one.
			name: "a slug containing a slash stays a single path segment",
			base: "https://live.rbg.tum.de",
			got:  func(c CanonicalURL) string { return c.Course(2023, "W", "ei/di") },
			want: "https://live.rbg.tum.de/course/2023/W/ei%2Fdi",
		},
		{
			// A crafted slug must not be able to point the canonical tag somewhere else.
			name: "a slug with .. cannot climb out of the course path",
			base: "https://live.rbg.tum.de",
			got:  func(c CanonicalURL) string { return c.Course(2023, "W", "../../../login") },
			want: "https://live.rbg.tum.de/course/2023/W/..%2F..%2F..%2Flogin",
		},
		{
			name: "a pile of .. segments no longer collapses the URL",
			base: "https://live.rbg.tum.de",
			got:  func(c CanonicalURL) string { return c.Course(2023, "W", "../../../../..") },
			want: "https://live.rbg.tum.de/course/2023/W/..%2F..%2F..%2F..%2F..",
		},
		{
			// url.PathEscape leaves a bare ".." alone, so it is encoded by hand.
			name: "a slug that is exactly .. is encoded rather than resolved",
			base: "https://live.rbg.tum.de",
			got:  func(c CanonicalURL) string { return c.Course(2023, "W", "..") },
			want: "https://live.rbg.tum.de/course/2023/W/%2E%2E",
		},
		{
			name: "a slug that is a single dot is encoded too",
			base: "https://live.rbg.tum.de",
			got:  func(c CanonicalURL) string { return c.Course(2023, "W", ".") },
			want: "https://live.rbg.tum.de/course/2023/W/%2E",
		},
		{
			// The course slug regex allows umlauts, which are percent-encoded here -
			// the escaped form is what a browser sends for the same link anyway.
			name: "a slug with a non-ASCII character is percent-encoded",
			base: "https://live.rbg.tum.de",
			got:  func(c CanonicalURL) string { return c.Course(2023, "W", "grüße") },
			want: "https://live.rbg.tum.de/course/2023/W/gr%C3%BC%C3%9Fe",
		},
		{
			name: "a slug with a space is percent-encoded rather than splitting the URL",
			base: "https://live.rbg.tum.de",
			got:  func(c CanonicalURL) string { return c.Course(2023, "W", "ei di") },
			want: "https://live.rbg.tum.de/course/2023/W/ei%20di",
		},
		{
			// A query or fragment marker in a slug must not end the path either.
			name: "a slug with ? or # stays inside the path",
			base: "https://live.rbg.tum.de",
			got:  func(c CanonicalURL) string { return c.Course(2023, "W", "ei?a#b") },
			want: "https://live.rbg.tum.de/course/2023/W/ei%3Fa%23b",
		},
		{
			name: "Stream joins slug, id and version",
			base: "https://live.rbg.tum.de",
			got:  func(c CanonicalURL) string { return c.Stream("eidi", 42, "CAM") },
			want: "https://live.rbg.tum.de/w/eidi/42/CAM",
		},
		{
			name: "Stream drops an empty version instead of leaving a trailing slash",
			base: "https://live.rbg.tum.de",
			got:  func(c CanonicalURL) string { return c.Stream("eidi", 42, "") },
			want: "https://live.rbg.tum.de/w/eidi/42",
		},
		{
			name: "Stream formats a large id without wrapping",
			base: "https://live.rbg.tum.de",
			got:  func(c CanonicalURL) string { return c.Stream("eidi", 4294967295, "PRES") },
			want: "https://live.rbg.tum.de/w/eidi/4294967295/PRES",
		},
		{
			name: "Stream on a base with a path prefix keeps the prefix",
			base: "https://example.org/tum",
			got:  func(c CanonicalURL) string { return c.Stream("eidi", 42, "COMB") },
			want: "https://example.org/tum/w/eidi/42/COMB",
		},
		{
			name: "Info appends the page name",
			base: "https://live.rbg.tum.de",
			got:  func(c CanonicalURL) string { return c.Info("imprint") },
			want: "https://live.rbg.tum.de/imprint",
		},
		{
			// Nothing to append, so the link is the origin itself, in its root form.
			name: "Info with an empty name falls back to the origin root",
			base: "https://live.rbg.tum.de",
			got:  func(c CanonicalURL) string { return c.Info("") },
			want: "https://live.rbg.tum.de/",
		},
		{
			name: "Info with an empty name on a base with a trailing slash keeps the origin",
			base: "https://live.rbg.tum.de/",
			got:  func(c CanonicalURL) string { return c.Info("") },
			want: "https://live.rbg.tum.de/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.got(NewCanonicalURL(tt.base)); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
