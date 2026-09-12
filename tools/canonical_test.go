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
			// path.Clean collapses the "//" in the scheme. Every method below inherits
			// this, so the canonical tags and share links ship a scheme-mangled URL.
			name: "a bare origin loses a slash from its scheme once anything is joined",
			base: "https://live.rbg.tum.de",
			got:  func(c CanonicalURL) string { return c.Login() },
			want: "https:/live.rbg.tum.de/login",
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
			want: "https:/live.rbg.tum.de/login",
		},
		{
			name: "a base with a path prefix keeps the prefix",
			base: "https://example.org/tum",
			got:  func(c CanonicalURL) string { return c.Login() },
			want: "https:/example.org/tum/login",
		},
		{
			// An unset canonicalURL in the config yields a relative path rather than
			// an absolute URL, which is not a valid canonical link.
			name: "an empty base produces a bare relative path",
			base: "",
			got:  func(c CanonicalURL) string { return c.Login() },
			want: "login",
		},
		{
			name: "Course joins year, term and slug",
			base: "https://live.rbg.tum.de",
			got:  func(c CanonicalURL) string { return c.Course(2023, "W", "eidi") },
			want: "https:/live.rbg.tum.de/course/2023/W/eidi",
		},
		{
			name: "Course renders a negative year rather than rejecting it",
			base: "https://live.rbg.tum.de",
			got:  func(c CanonicalURL) string { return c.Course(-1, "S", "eidi") },
			want: "https:/live.rbg.tum.de/course/-1/S/eidi",
		},
		{
			// Slugs come from user-editable course settings, so what a separator in one
			// does to the resulting URL is pinned deliberately.
			name: "a slug containing a slash becomes an extra path segment",
			base: "https://live.rbg.tum.de",
			got:  func(c CanonicalURL) string { return c.Course(2023, "W", "ei/di") },
			want: "https:/live.rbg.tum.de/course/2023/W/ei/di",
		},
		{
			// path.Clean resolves the traversal instead of leaving it in the link, so a
			// crafted slug can point the canonical tag at an unrelated path.
			name: "a slug with .. is cleaned away and climbs out of the course path",
			base: "https://live.rbg.tum.de",
			got:  func(c CanonicalURL) string { return c.Course(2023, "W", "../../../login") },
			want: "https:/live.rbg.tum.de/login",
		},
		{
			name: "enough .. segments collapse the whole URL to the current directory",
			base: "https://live.rbg.tum.de",
			got:  func(c CanonicalURL) string { return c.Course(2023, "W", "../../../../..") },
			want: ".",
		},
		{
			name: "Stream joins slug, id and version",
			base: "https://live.rbg.tum.de",
			got:  func(c CanonicalURL) string { return c.Stream("eidi", 42, "CAM") },
			want: "https:/live.rbg.tum.de/w/eidi/42/CAM",
		},
		{
			name: "Stream drops an empty version instead of leaving a trailing slash",
			base: "https://live.rbg.tum.de",
			got:  func(c CanonicalURL) string { return c.Stream("eidi", 42, "") },
			want: "https:/live.rbg.tum.de/w/eidi/42",
		},
		{
			// The id is a uint but is narrowed through int before formatting.
			name: "Stream formats a large id without wrapping",
			base: "https://live.rbg.tum.de",
			got:  func(c CanonicalURL) string { return c.Stream("eidi", 4294967295, "PRES") },
			want: "https:/live.rbg.tum.de/w/eidi/4294967295/PRES",
		},
		{
			name: "Info appends the page name",
			base: "https://live.rbg.tum.de",
			got:  func(c CanonicalURL) string { return c.Info("imprint") },
			want: "https:/live.rbg.tum.de/imprint",
		},
		{
			name: "Info with an empty name falls back to the cleaned origin",
			base: "https://live.rbg.tum.de",
			got:  func(c CanonicalURL) string { return c.Info("") },
			want: "https:/live.rbg.tum.de",
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
