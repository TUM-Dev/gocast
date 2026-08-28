package model

// Permission names something a user may do.
//
// Call sites state the capability they need rather than the role that happens to have
// it, so granting a capability to a new role is an edit to rolePermissions below
// instead of a search for every `Role ==` in the tree. The roles are a linear rank
// numbered opposite to authority (AdminType = 1 … StudentType = 4), which makes
// direct comparisons easy to get backwards.
type Permission string

const (
	// PermAdministerServer covers operations belonging to no course: lecture halls,
	// runners, workers, maintenance, notifications, audits, info pages, statistics.
	PermAdministerServer Permission = "server.administer"

	// PermAdministerAllCourses grants course administration without an explicit
	// grant. It is the wildcard half of CanAdminister.
	PermAdministerAllCourses Permission = "courses.administer.all"

	// PermViewAllCourses grants sight of every course and stream whatever its
	// visibility. Separate from PermAdministerServer on purpose: running the
	// infrastructure does not require reading every recorded lecture.
	PermViewAllCourses Permission = "courses.view.all"

	// PermManageUsers grants administration of accounts other than one's own,
	// including other users' API tokens and minting admin-scoped ones.
	PermManageUsers Permission = "users.manage"

	// PermLecture grants lecturer capabilities not scoped to an existing course:
	// creating courses and obtaining a stream key. What AtLeastLecturer meant.
	PermLecture Permission = "lecture"
)

// rolePermissions maps a role to what it may do. It reproduces exactly the authority
// the roles hold today: an admin is a lecturer for every course plus server
// administration; a lecturer administers only what they are granted.
//
// Adding a role means adding a row here, and nothing else should compare against Role.
var rolePermissions = map[uint][]Permission{
	AdminType: {
		PermAdministerServer,
		PermAdministerAllCourses,
		PermViewAllCourses,
		PermManageUsers,
		PermLecture,
	},
	LecturerType: {
		PermLecture,
	},
	GenericType: {},
	StudentType: {},
}

// Can reports whether the user holds a permission.
//
// A nil user is anonymous and holds nothing, so callers can ask before establishing
// that anyone is signed in. An unmapped role — including the zero value — holds
// nothing either: an unknown role must not be a way to gain authority.
func (u *User) Can(p Permission) bool {
	if u == nil {
		return false
	}

	for _, held := range rolePermissions[u.Role] {
		if held == p {
			return true
		}
	}

	return false
}

// CanAdminister reports whether the user may administer a course, by holding the
// wildcard permission, by having been granted the course explicitly, or by owning it.
func (u *User) CanAdminister(c Course) bool {
	if u == nil {
		return false
	}

	if u.Can(PermAdministerAllCourses) {
		return true
	}

	for _, administered := range u.AdministeredCourses {
		if administered.ID == c.ID {
			return true
		}
	}

	// A user with no ID was never persisted, so it owns nothing. Without this the
	// zero value matches every course whose owner is unset.
	if u.ID == 0 {
		return false
	}

	return c.UserID == u.ID
}
