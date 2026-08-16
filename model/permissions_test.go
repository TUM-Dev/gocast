package model

import (
	"testing"

	"gorm.io/gorm"
)

// The mapping is the whole authorization model in one table, so it is asserted
// exhaustively. A permission quietly spreading to another role is invisible to a test
// that only checks the grants it expects to find.
func TestRolePermissions(t *testing.T) {
	all := []Permission{
		PermAdministerServer,
		PermAdministerAllCourses,
		PermViewAllCourses,
		PermManageUsers,
		PermLecture,
	}

	granted := map[uint]map[Permission]bool{
		AdminType: {
			PermAdministerServer:     true,
			PermAdministerAllCourses: true,
			PermViewAllCourses:       true,
			PermManageUsers:          true,
			PermLecture:              true,
		},
		LecturerType: {
			PermLecture: true,
		},
		GenericType: {},
		StudentType: {},
		// The zero value a partially loaded user carries: it must grant nothing.
		0: {},
	}

	for role, want := range granted {
		for _, p := range all {
			user := &User{Role: role}
			if got := user.Can(p); got != want[p] {
				t.Errorf("role %d: Can(%q) = %v, want %v", role, p, got, want[p])
			}
		}
	}

	t.Run("an anonymous caller holds nothing", func(t *testing.T) {
		var user *User
		for _, p := range all {
			if user.Can(p) {
				t.Errorf("nil user was granted %q", p)
			}
		}
	})
}

// CanAdminister is where the wildcard and the per-course grant meet, and it gates
// every course administration route, so each way to say yes is pinned separately.
func TestCanAdminister(t *testing.T) {
	course := Course{Model: gorm.Model{ID: 7}, UserID: 42}
	other := Course{Model: gorm.Model{ID: 8}, UserID: 42}

	tests := []struct {
		name string
		user *User
		want bool
	}{
		{
			name: "an admin administers any course without a grant",
			user: &User{Model: gorm.Model{ID: 1}, Role: AdminType},
			want: true,
		},
		{
			name: "a lecturer administers a course granted to them",
			user: &User{
				Model:               gorm.Model{ID: 2},
				Role:                LecturerType,
				AdministeredCourses: []Course{course},
			},
			want: true,
		},
		{
			name: "the owner administers their own course",
			user: &User{Model: gorm.Model{ID: 42}, Role: LecturerType},
			want: true,
		},
		{
			name: "a lecturer granted a different course does not",
			user: &User{
				Model:               gorm.Model{ID: 3},
				Role:                LecturerType,
				AdministeredCourses: []Course{other},
			},
			want: false,
		},
		{
			name: "a student does not",
			user: &User{Model: gorm.Model{ID: 4}, Role: StudentType},
			want: false,
		},
		{
			name: "an anonymous caller does not",
			user: nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.user.CanAdminister(course); got != tt.want {
				t.Errorf("CanAdminister = %v, want %v", got, tt.want)
			}
			// IsAdminOfCourse delegates here; this catches a reimplementation.
			if got := tt.user.IsAdminOfCourse(course); got != tt.want {
				t.Errorf("IsAdminOfCourse = %v, want %v", got, tt.want)
			}
		})
	}
}
