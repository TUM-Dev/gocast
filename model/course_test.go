package model

import (
	"strings"
	"testing"
)

func TestBeforeSave(t *testing.T) {
	course := &Course{}
	cases := []struct {
		slug    string
		wantErr bool
	}{
		{"", true},
		{strings.Repeat("a", 300), true},
		{"test123", false},
		{"test_123", false},
		{"!test", true},
		{"\" && rm -rf /", true},
	}
	for _, tc := range cases {
		course.Slug = tc.slug
		err := course.BeforeSave(nil)
		if err == nil && tc.wantErr {
			t.Errorf("BeforeCreate(%q) = nil, want error", tc.slug)
		} else if err != nil && !tc.wantErr {
			t.Errorf("BeforeCreate(%q) = %v, want nil", tc.slug, err)
		}
	}
}
