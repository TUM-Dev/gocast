package helpers

import (
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/TUM-Dev/gocast/model"
)

// The preferred name is the one setting with rules of its own, and it is now only
// reachable through the v2 endpoint — the v1 handler that carried these checks was
// deleted. Both rules were lost in the port.
func TestValidatePreferredName(t *testing.T) {
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)

	setting := func(updated time.Time) *model.UserSetting {
		s := &model.UserSetting{Type: model.PreferredName, Value: "old"}
		// CreatedAt stays at the first write; the cooldown must not read it.
		s.CreatedAt = now.Add(-5 * 365 * 24 * time.Hour)
		s.UpdatedAt = updated
		return s
	}

	tests := []struct {
		name       string
		value      string
		lastChange *model.UserSetting
		wantCode   codes.Code
	}{
		{
			name:  "a first name may be set",
			value: "Hansi",
		},
		{
			name:     "an empty name is refused",
			value:    "",
			wantCode: codes.InvalidArgument,
		},
		{
			name:  "a name of the maximum length is accepted",
			value: strings.Repeat("a", model.MaxUsernameLength),
		},
		{
			// The column is varchar(80), so without this the write fails in the
			// driver instead of returning a clean 400.
			name:     "a name over the maximum length is refused",
			value:    strings.Repeat("a", model.MaxUsernameLength+1),
			wantCode: codes.InvalidArgument,
		},
		{
			name:       "a change within three months is refused",
			value:      "Hansi",
			lastChange: setting(now.Add(-30 * 24 * time.Hour)),
			wantCode:   codes.InvalidArgument,
		},
		{
			// The case CreatedAt got wrong: a row created years ago but updated
			// last month is still inside its cooldown.
			name:       "an old row updated recently is still in cooldown",
			value:      "Hansi",
			lastChange: setting(now.Add(-24 * time.Hour)),
			wantCode:   codes.InvalidArgument,
		},
		{
			name:       "a change after three months is allowed",
			value:      "Hansi",
			lastChange: setting(now.Add(-100 * 24 * time.Hour)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePreferredName(tt.value, tt.lastChange, now)

			if tt.wantCode == codes.OK {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if got := status.Code(err); got != tt.wantCode {
				t.Errorf("code = %v, want %v (err %v)", got, tt.wantCode, err)
			}
		})
	}
}
