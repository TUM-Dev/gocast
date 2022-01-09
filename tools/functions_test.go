package tools

import (
	"testing"
)

func TestMaskEmail(t *testing.T) {
	email := "charly@example.com"
	expected := "c*****@example.com"
	masked, err := MaskEmail(email)
	if err != nil {
		t.Errorf("MaskEmail(%s) failed: %s", email, err)
	}
	if masked != expected {
		t.Errorf("MaskEmail(%s) = %s, want %s", email, masked, expected)
	}
}

func TestMaskEmailInvalid(t *testing.T) {
	vals := []string{"charly@", "charly@e", "@example.com", "charlyexample.com", ""}
	for _, val := range vals {
		masked, err := MaskEmail(val)
		if err == nil {
			t.Errorf("MaskEmail(%s) should fail but returned: %s", val, masked)
		}
	}
}

func TestMaskLogin(t *testing.T) {
	exp := "ge**tum"
	if masked := MaskLogin("ge12tum"); masked != exp {
		t.Errorf("MaskLogin(ge12tum) = %s, want %s", masked, "ge**tum")
	}
}
