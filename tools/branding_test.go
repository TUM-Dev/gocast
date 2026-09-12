package tools

import (
	"os"
	"path/filepath"
	"testing"
)

// The defaults are what the site header renders whenever branding.yaml is absent, which
// is the normal case for a fresh deployment, so a blank field here ships a blank header.
func TestGetDefaultBranding(t *testing.T) {
	b := getDefaultBranding()

	if b.Title != "TUM-Live" {
		t.Errorf("Title = %q, want %q", b.Title, "TUM-Live")
	}
	if b.Description == "" {
		t.Error("Description is empty, want the default description")
	}
}

// InitBranding writes the package global BrandingCfg and reads a config file from the
// working directory, so both are restored around every case.
func TestInitBranding(t *testing.T) {
	t.Run("falls back to the defaults when there is no branding file", func(t *testing.T) {
		withIsolatedBrandingEnv(t)

		InitBranding()

		if BrandingCfg != getDefaultBranding() {
			t.Errorf("BrandingCfg = %+v, want the defaults", BrandingCfg)
		}
	})

	t.Run("takes title and description from branding.yaml", func(t *testing.T) {
		dir := withIsolatedBrandingEnv(t)
		writeBrandingYAML(t, dir, "title: GoCast\ndescription: lecture streaming\n")

		InitBranding()

		if BrandingCfg.Title != "GoCast" {
			t.Errorf("Title = %q, want %q", BrandingCfg.Title, "GoCast")
		}
		if BrandingCfg.Description != "lecture streaming" {
			t.Errorf("Description = %q, want %q", BrandingCfg.Description, "lecture streaming")
		}
	})

	// A partial file must not blank out the fields it leaves unset: unmarshalling runs
	// on top of the defaults rather than on a zero value.
	t.Run("a partial file keeps the default for the fields it omits", func(t *testing.T) {
		dir := withIsolatedBrandingEnv(t)
		writeBrandingYAML(t, dir, "title: GoCast\n")

		InitBranding()

		if BrandingCfg.Title != "GoCast" {
			t.Errorf("Title = %q, want %q", BrandingCfg.Title, "GoCast")
		}
		if BrandingCfg.Description != getDefaultBranding().Description {
			t.Errorf("Description = %q, want the default", BrandingCfg.Description)
		}
	})

	t.Run("an empty file leaves the defaults in place", func(t *testing.T) {
		dir := withIsolatedBrandingEnv(t)
		writeBrandingYAML(t, dir, "")

		InitBranding()

		if BrandingCfg != getDefaultBranding() {
			t.Errorf("BrandingCfg = %+v, want the defaults", BrandingCfg)
		}
	})
}

// withIsolatedBrandingEnv points every search path InitBranding consults at a temporary
// directory and restores BrandingCfg afterwards, so nothing here reaches the real
// machine or leaks into the rest of the package.
func withIsolatedBrandingEnv(t *testing.T) string {
	t.Helper()

	// InitBranding also searches /etc/TUM-Live, which a test cannot isolate.
	if _, err := os.Stat("/etc/TUM-Live/branding.yaml"); err == nil {
		t.Skip("a machine-wide /etc/TUM-Live/branding.yaml would win over the test fixture")
	}

	dir := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Chdir(dir)

	before := BrandingCfg
	t.Cleanup(func() { BrandingCfg = before })

	return dir
}

func writeBrandingYAML(t *testing.T, dir, content string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(dir, "branding.yaml"), []byte(content), 0o600); err != nil {
		t.Fatalf("writing branding.yaml: %v", err)
	}
}
