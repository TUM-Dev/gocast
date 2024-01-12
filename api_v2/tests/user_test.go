package tests

import (
	"context"
	"fmt"
	"testing"

	"github.com/TUM-Dev/gocast/api_v2/protobuf"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// TestGetUser tests the GetUser function.
func TestGetUser_LoggedIn(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), md_student_loggedin)
	req := &protobuf.GetUserRequest{}
	user, err := a.GetUser(ctx, req)
	fmt.Println(user)
	if status.Code(err) != codes.OK {
		t.Errorf("expected OK, got %v", err)
	}

	if user.User.Id != 7 {
		t.Errorf("expected 7, got %v", user.User.Id)
	}

	if user.User.Name != "LoggedInUser" {
		t.Errorf("expected LoggedInUser, got %v", user.User.Name)
	}

	if user.User.LastName != "log" {
		t.Errorf("expected log, got %v", user.User.LastName)
	}

	if user.User.Email != "loggedin" {
		t.Errorf("expected loggedin, got %v", user.User.Email)
	}

	if user.User.Role != 4 {
		t.Errorf("expected 4, got %v", user.User.Role)
	}

}

func TestGetUser_InvalidJWT(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), md_invalid_jwt)
	req := &protobuf.GetUserRequest{}
	_, err := a.GetUser(ctx, req)
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated, got %v", err)
	}
}

// Test Get User Courses
func TestGetUserCourses_NoCourses(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), md_student_loggedin)
	req := &protobuf.GetUserCoursesRequest{}
	courses, err := a.GetUserCourses(ctx, req)

	if status.Code(err) != codes.OK {
		t.Errorf("expected OK, got %v", err)
	}

	if len(courses.Courses) != 0 {
		t.Errorf("expected 0, got %v", len(courses.Courses))
	}
}

func TestGetUserCourses_Enrolled(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), md_student_enrolled)
	req := &protobuf.GetUserCoursesRequest{}
	courses, err := a.GetUserCourses(ctx, req)

	fmt.Println(courses)

	if status.Code(err) != codes.OK {
		t.Errorf("expected OK, got %v", err)
	}

	if len(courses.Courses) != 5 {
		t.Errorf("expected 5, got %v", len(courses.Courses))
	}

	any := false
	for _, c := range courses.Courses {
		if c.Id == 2 {
			any = true
		}
	}
	if !any {
		t.Errorf("expected to be enrolled in id: 2, got %v", courses.Courses)
	}
}

func TestGetUserCourses_InvalidJWT(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), md_invalid_jwt)
	req := &protobuf.GetUserCoursesRequest{}
	_, err := a.GetUserCourses(ctx, req)
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated, got %v", err)
	}
}

// Test Get User Pinned
func TestGetUserPinned_NoCourses(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), md_student_loggedin)
	req := &protobuf.GetUserPinnedRequest{}
	courses, err := a.GetUserPinned(ctx, req)

	if status.Code(err) != codes.OK {
		t.Errorf("expected OK, got %v", err)
	}

	if len(courses.Courses) != 0 {
		t.Errorf("expected 0, got %v", len(courses.Courses))
	}
}

func TestGetUserPinned_Enrolled(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), md_student_enrolled)
	req := &protobuf.GetUserPinnedRequest{}
	courses, err := a.GetUserPinned(ctx, req)

	if status.Code(err) != codes.OK {
		t.Errorf("expected OK, got %v", err)
	}

	if len(courses.Courses) != 0 {
		t.Errorf("expected 0, got %v", len(courses.Courses))
	}
}

func TestGetUserPinned_InvalidJWT(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), md_invalid_jwt)
	req := &protobuf.GetUserPinnedRequest{}
	_, err := a.GetUserPinned(ctx, req)
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated, got %v", err)
	}
}

// Test Get User Bookmarks
func TestGetUserBookmarks_NoCourses(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), md_student_loggedin)
	req := &protobuf.GetBookmarksRequest{}
	courses, err := a.GetUserBookmarks(ctx, req)

	if status.Code(err) != codes.OK {
		t.Errorf("expected OK, got %v", err)
	}

	if len(courses.Bookmarks) != 0 {
		t.Errorf("expected 0, got %v", len(courses.Bookmarks))
	}
}

func TestGetUserBookmarks_Enrolled(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), md_student_enrolled)
	req := &protobuf.GetBookmarksRequest{}
	courses, err := a.GetUserBookmarks(ctx, req)

	if status.Code(err) != codes.OK {
		t.Errorf("expected OK, got %v", err)
	}

	if len(courses.Bookmarks) != 0 {
		t.Errorf("expected 0, got %v", len(courses.Bookmarks))
	}
}

func TestGetUserBookmarks_InvalidJWT(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), md_invalid_jwt)
	req := &protobuf.GetBookmarksRequest{}
	_, err := a.GetUserBookmarks(ctx, req)
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated, got %v", err)
	}
}

// Test Put User Bookmarks
func TestPutUserBookmarks_InvalidJWT(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), md_invalid_jwt)
	req := &protobuf.PutBookmarkRequest{}
	_, err := a.PutUserBookmark(ctx, req)
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated, got %v", err)
	}
}

func TestPutUserBookmarks_Enrolled(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), md_student_enrolled)
	req := &protobuf.PutBookmarkRequest{
		Description: "test",
		Hours:       1,
		Minutes:     1,
		Seconds:     1,
		StreamID:    1,
	}

	bookmark, err := a.PutUserBookmark(ctx, req)

	if status.Code(err) != codes.OK {
		t.Errorf("expected OK, got %v", err)
	}

	if bookmark.Bookmark.Description != "test" {
		t.Errorf("expected test, got %v", bookmark.Bookmark.Description)
	}

	if bookmark.Bookmark.Hours != 1 {
		t.Errorf("expected 1, got %v", bookmark.Bookmark.Hours)
	}
}
