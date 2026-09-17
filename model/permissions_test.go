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

	// Ownership matches on ID and both halves are zero here, which is why this needs
	// its own case.
	t.Run("an unpersisted user does not administer an unowned course", func(t *testing.T) {
		unowned := Course{Model: gorm.Model{ID: 9}}
		if (&User{}).CanAdminister(unowned) {
			t.Error("the zero value of a user administers a course with no owner")
		}
	})
}

// Permissions is what a client is told; Can is what the server enforces. Drift
// between them offers controls that every request behind them refuses.
func TestPermissionsAgreesWithCan(t *testing.T) {
	all := []Permission{
		PermAdministerServer,
		PermAdministerAllCourses,
		PermViewAllCourses,
		PermManageUsers,
		PermLecture,
	}

	// The zero value included: a partially loaded user must be told it holds nothing.
	for _, role := range []uint{AdminType, LecturerType, GenericType, StudentType, 0} {
		user := &User{Role: role}

		listed := map[Permission]bool{}
		for _, p := range user.Permissions() {
			listed[p] = true
		}

		for _, p := range all {
			if listed[p] != user.Can(p) {
				t.Errorf("role %d: Permissions() lists %q = %v, Can says %v", role, p, listed[p], user.Can(p))
			}
		}
	}
}

// Anonymous callers reach the parser too, through the endpoints that serve them.
func TestPermissionsOfAnonymousUser(t *testing.T) {
	var user *User

	if got := user.Permissions(); len(got) != 0 {
		t.Errorf("Permissions() = %v, want none", got)
	}
}

// Returning the table itself would let a caller corrupt the authorization model.
func TestPermissionsDoesNotExposeTheTable(t *testing.T) {
	user := &User{Role: LecturerType}

	got := user.Permissions()
	if len(got) == 0 {
		t.Fatal("a lecturer holds no permissions at all")
	}
	got[0] = "something.else"

	if !user.Can(PermLecture) {
		t.Error("editing the returned slice changed what the role may do")
	}
}
