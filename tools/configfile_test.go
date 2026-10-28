package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func isolatedConfigDir(t *testing.T) string {
	t.Helper()
	if _, err := os.Stat("/etc/TUM-Live"); err == nil {
		t.Skip("a machine-wide /etc/TUM-Live could win over the test fixture")
	}
	dir := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	t.Chdir(dir)
	return dir
}

// The deployed config files spell keys in any casing ("camauth", "jwtkey", "voiceservice")
// and by Go field name ("rootURLs"), which is what viper's mapstructure matching allowed.
func TestLoadConfigFileKeyMatching(t *testing.T) {
	dir := isolatedConfigDir(t)
	content := "DB:\n  Port: \"3306\"\n  host: h\nsaml:\n  rootURLs: [a, b]\n  idpName: x\ncampus:\n  relevantorgs: [14189]\nvoiceservice:\n  host: v\nauths:\n  camauth: u:p\n"
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	var cfg Config
	if _, err := loadConfigFile("config", &cfg); err != nil {
		t.Fatal(err)
	}

	switch {
	case cfg.Db.Port != 3306 || cfg.Db.Host != "h":
		t.Errorf("Db = %+v, want port 3306 (quoted) and host h", cfg.Db)
	case cfg.Saml == nil || len(cfg.Saml.RootURLs) != 2 || cfg.Saml.IdpName != "x":
		t.Errorf("Saml = %+v, want two rootURLs and idpName x", cfg.Saml)
	case cfg.Campus.RelevantOrgs == nil || (*cfg.Campus.RelevantOrgs)[0] != "14189":
		t.Errorf("RelevantOrgs = %v, want the int rendered as a string", cfg.Campus.RelevantOrgs)
	case cfg.VoiceService == nil || cfg.VoiceService.Host != "v":
		t.Errorf("VoiceService = %+v, want host v", cfg.VoiceService)
	case cfg.Auths.CamAuth != "u:p":
		t.Errorf("CamAuth = %q, want u:p", cfg.Auths.CamAuth)
	}
}

func TestLoadConfigFileMissing(t *testing.T) {
	isolatedConfigDir(t)
	var cfg Config
	if _, err := loadConfigFile("config", &cfg); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("err = %v, want a not-found error", err)
	}
}

func TestLoadConfigFileInvalid(t *testing.T) {
	dir := isolatedConfigDir(t)
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("db: [unclosed"), 0o600); err != nil {
		t.Fatal(err)
	}
	var cfg Config
	if _, err := loadConfigFile("config", &cfg); err == nil {
		t.Error("want a parse error, got nil")
	}
}

// setConfigKey must keep everything else in the file: operators hand-edit it and comment it.
func TestSetConfigKey(t *testing.T) {
	dir := isolatedConfigDir(t)
	path := filepath.Join(dir, "config.yaml")
	original := "# keep me\ndb:\n  host: h # and me\nJWTKey: old\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}

	key := "-----BEGIN RSA PRIVATE KEY-----\nabc\n-----END RSA PRIVATE KEY-----\n"
	if err := setConfigKey(path, "jwtKey", key); err != nil {
		t.Fatal(err)
	}

	raw, _ := os.ReadFile(path)
	for _, want := range []string{"# keep me", "# and me", "host: h"} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("rewritten file lost %q:\n%s", want, raw)
		}
	}
	if strings.Contains(string(raw), "old") || strings.Count(strings.ToLower(string(raw)), "jwtkey") != 1 {
		t.Errorf("the existing key must be replaced in place:\n%s", raw)
	}
	var cfg Config
	if _, err := loadConfigFile("config", &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.JWTKey == nil || *cfg.JWTKey != key {
		t.Errorf("JWTKey after round trip = %v, want %q", cfg.JWTKey, key)
	}

	// A file without the key gets it appended.
	if err := os.WriteFile(path, []byte("db:\n  host: h\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := setConfigKey(path, "jwtKey", "k"); err != nil {
		t.Fatal(err)
	}
	cfg = Config{}
	if _, err := loadConfigFile("config", &cfg); err != nil || cfg.JWTKey == nil || *cfg.JWTKey != "k" || cfg.Db.Host != "h" {
		t.Errorf("append case: cfg = %+v, err = %v", cfg, err)
	}
}
