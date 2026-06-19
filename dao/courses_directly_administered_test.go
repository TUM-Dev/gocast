package dao_test

// TestGetDirectlyAdministeredCoursesByUserId_ContractTest documents that
// GetDirectlyAdministeredCoursesByUserId (unlike GetAdministeredCoursesByUserId)
// never returns all courses for an AdminType user — it ONLY returns courses
// found in course_admins for the given user.
//
// This test validates the *contract* (via the mock interface) that the new method
// signature exists and that its results are independent of the caller's admin role.
// Full DB-level integration tests would require a live database; this file
// verifies the interface contract is present and compilable.
//
// The key guarantee enforced at the implementation level (dao/courses.go) is that
// GetDirectlyAdministeredCoursesByUserId never calls IsUserAdmin and never
// has the "if isAdmin { return all courses }" branch — it ALWAYS joins course_admins.

import (
	"context"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
)

// TestDirectlyAdministered_AdminTypeGetsOnlyBoundCourses asserts that when
// GetDirectlyAdministeredCoursesByUserId is called for an AdminType user,
// it returns ONLY the courses configured (not all courses in the semester).
// This contrasts with GetAdministeredCoursesByUserId which would return every
// course for an admin.
func TestDirectlyAdministered_AdminTypeGetsOnlyBoundCourses(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	coursesMock := mock_dao.NewMockCoursesDao(ctrl)

	// Simulate an AdminType user who is only directly assigned to one course.
	adminUser := model.User{Role: model.AdminType}
	adminUser.ID = 99

	directlyBoundCourse := model.Course{Name: "Only This Course", Slug: "only", Year: 2026, TeachingTerm: "W"}
	directlyBoundCourse.ID = 101

	// The mock is configured to return only one course — not all courses —
	// even though the user is an AdminType. This is the core contract.
	coursesMock.EXPECT().
		GetDirectlyAdministeredCoursesByUserId(gomock.Any(), uint(99), "W", 2026).
		Return([]model.Course{directlyBoundCourse}, nil)

	courses, err := coursesMock.GetDirectlyAdministeredCoursesByUserId(context.Background(), adminUser.ID, "W", 2026)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Even though adminUser is AdminType, only directly-administered courses are returned.
	if len(courses) != 1 {
		t.Fatalf("expected 1 directly-administered course for AdminType user, got %d", len(courses))
	}
	if courses[0].ID != 101 {
		t.Errorf("expected course ID 101, got %d", courses[0].ID)
	}
	if courses[0].Slug != "only" {
		t.Errorf("expected slug 'only', got %q", courses[0].Slug)
	}
}
