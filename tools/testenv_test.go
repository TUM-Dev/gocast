package tools

import "testing"

func TestTestEnvPolicy(t *testing.T) {
	t.Run("zero value allows everything", func(t *testing.T) {
		var p TestEnvPolicy
		if !p.AllowsStream(42) {
			t.Error("production policy must allow every stream")
		}
	})

	t.Run("test env without courses allows none", func(t *testing.T) {
		p, err := NewTestEnvPolicy(true, "")
		if err != nil {
			t.Fatal(err)
		}
		if p.AllowsStream(1) {
			t.Error("no course is allowlisted, stream must be refused")
		}
	})

	t.Run("test env allows only listed courses", func(t *testing.T) {
		p, err := NewTestEnvPolicy(true, "1, 7")
		if err != nil {
			t.Fatal(err)
		}
		if !p.AllowsStream(1) || !p.AllowsStream(7) {
			t.Error("listed courses must be allowed")
		}
		if p.AllowsStream(2) {
			t.Error("unlisted course must be refused")
		}
	})

	t.Run("courses without test env are an error", func(t *testing.T) {
		if _, err := NewTestEnvPolicy(false, "1"); err == nil {
			t.Error("want an error, a stray flag must not look like a restriction")
		}
	})

	t.Run("invalid course ID is an error", func(t *testing.T) {
		for _, in := range []string{"abc", "1,,2", "-3"} {
			if _, err := NewTestEnvPolicy(true, in); err == nil {
				t.Errorf("NewTestEnvPolicy(true, %q) = nil error, want one", in)
			}
		}
	})
}
