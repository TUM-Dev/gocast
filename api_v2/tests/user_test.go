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

// test post user pinned and delete user pinned
func TestPostUserPinned_InvalidJWT(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), md_invalid_jwt)
	req := &protobuf.PostPinnedRequest{
		CourseID: 1,
	}
	_, err := a.PostUserPinned(ctx, req)
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated, got %v", err)
	}
}

func TestPostUserPinned_Enrolled(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), md_student_enrolled)
	req := &protobuf.PostPinnedRequest{
		CourseID: 1,
	}
	_, err := a.PostUserPinned(ctx, req)
	if status.Code(err) != codes.OK {
		t.Errorf("expected OK, got %v", err)
	}

	// check if it is pinned
	courses, err := a.GetUserPinned(ctx, &protobuf.GetUserPinnedRequest{})
	if status.Code(err) != codes.OK {
		t.Errorf("expected OK, got %v", err)
	}

	any := false
	for _, c := range courses.Courses {
		if c.Id == 1 {
			any = true
		}
	}
	if !any {
		t.Errorf("expected to be pinned in id: 1, got %v", courses.Courses)
	}
}

func TestDeleteUserPinned_InvalidJWT(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), md_invalid_jwt)
	req := &protobuf.DeletePinnedRequest{
		CourseID: 1,
	}
	_, err := a.DeleteUserPinned(ctx, req)
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated, got %v", err)
	}
}

func TestDeleteUserPinned_Enrolled(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), md_student_enrolled)
	//  first create one to delete
	postReq := &protobuf.PostPinnedRequest{
		CourseID: 3,
	}

	_, err := a.PostUserPinned(ctx, postReq)
	if status.Code(err) != codes.OK {
		t.Errorf("expected OK, got %v", err)
	}

	// delete it
	deleteReq := &protobuf.DeletePinnedRequest{
		CourseID: 3,
	}

	_, err = a.DeleteUserPinned(ctx, deleteReq)
	if status.Code(err) != codes.OK {
		t.Errorf("expected OK, got %v", err)
	}

	// check if it is deleted
	courses, err := a.GetUserPinned(ctx, &protobuf.GetUserPinnedRequest{})
	if status.Code(err) != codes.OK {
		t.Errorf("expected OK, got %v", err)
	}

	any := false
	for _, c := range courses.Courses {
		if c.Id == 1 {
			any = true
		}
	}
	if any {
		t.Errorf("expected to be deleted in id: 1, got %v", courses.Courses)
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

// Patch user bookmark for this put one to know what to patch
func TestPatchUserBookmarks_InvalidJWT(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), md_invalid_jwt)
	req := &protobuf.PatchBookmarkRequest{}
	_, err := a.PatchUserBookmark(ctx, req)
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated, got %v", err)
	}
}

func TestPatchUserBookmarks_Enrolled(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), md_student_enrolled)

	// put one to know what to patch
	putReq := &protobuf.PutBookmarkRequest{
		Description: "test",
		Hours:       1,
		Minutes:     1,
		Seconds:     1,
		StreamID:    1,
	}

	res, _ := a.PutUserBookmark(ctx, putReq)
	// handling error is not necessary because we know it works from the previous test

	req := &protobuf.PatchBookmarkRequest{
		BookmarkID:  res.Bookmark.Id,
		Description: "test",
		Hours:       1,
		Minutes:     1,
		Seconds:     1,
	}

	bookmark, err := a.PatchUserBookmark(ctx, req)

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

// Delete user bookmark for this put one to know what to delete
func TestDeleteUserBookmarks_InvalidJWT(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), md_invalid_jwt)
	req := &protobuf.DeleteBookmarkRequest{}
	_, err := a.DeleteUserBookmark(ctx, req)
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated, got %v", err)
	}
}

func TestDeleteUserBookmarks_Enrolled(t *testing.T) {
	// put one to know what to delete
	ctx := metadata.NewIncomingContext(context.Background(), md_student_enrolled)
	putReq := &protobuf.PutBookmarkRequest{
		Description: "test",
		Hours:       1,
		Minutes:     1,
		Seconds:     1,
		StreamID:    1,
	}

	res, _ := a.PutUserBookmark(ctx, putReq)
	// handling error is not necessary because we know it works from the previous test

	req := &protobuf.DeleteBookmarkRequest{
		BookmarkID: res.Bookmark.Id,
	}

	_, err := a.DeleteUserBookmark(ctx, req)

	if status.Code(err) != codes.OK {
		t.Errorf("expected OK, got %v", err)
	}

	// check if it is deleted
	_, err = a.DeleteUserBookmark(ctx, req)
	if status.Code(err) != codes.NotFound {
		t.Errorf("expected NotFound, got %v", err)
	}
}

// Test Settings
func TestGetUserSettings_InvalidJWT(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), md_invalid_jwt)
	req := &protobuf.GetUserSettingsRequest{}
	_, err := a.GetUserSettings(ctx, req)
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated, got %v", err)
	}
}

func TestGetUserSettings_Enrolled_EmptySettings(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), md_student_enrolled)
	req := &protobuf.GetUserSettingsRequest{}
	settings, err := a.GetUserSettings(ctx, req)

	fmt.Println("settings")
	fmt.Println(settings)

	if status.Code(err) != codes.OK {
		t.Errorf("expected OK, got %v", err)
	}
}

// test patch user settings
func TestPatchUserSettings_InvalidJWT(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), md_invalid_jwt)
	req := &protobuf.PatchUserSettingsRequest{}
	_, err := a.PatchUserSettings(ctx, req)
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated, got %v", err)
	}
}

func TestPatchUserSettings_Enrolled_ChangeNameValid(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), md_student_enrolled)
	req := &protobuf.PatchUserSettingsRequest{
		UserSettings: []*protobuf.UserSetting{
			{
				Type:  protobuf.UserSettingType_PREFERRED_NAME,
				Value: "test",
			},
		},
	}

	settings, err := a.PatchUserSettings(ctx, req)

	fmt.Println("settings")
	fmt.Println(settings)

	if status.Code(err) != codes.OK {
		t.Errorf("expected OK, got %v", err)
	}
}

func TestPatchUserSettings_Enrolled_ChangeNameEmpty(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), md_student_enrolled)
	req := &protobuf.PatchUserSettingsRequest{
		UserSettings: []*protobuf.UserSetting{
			{
				Type:  protobuf.UserSettingType_PREFERRED_NAME,
				Value: "",
			},
		},
	}

	_, err := a.PatchUserSettings(ctx, req)

	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", err)
	}
}

func TestPatchUserSettings_Enrolled_ChangeNameTwice(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), md_student_enrolled)
	req := &protobuf.PatchUserSettingsRequest{
		UserSettings: []*protobuf.UserSetting{
			{
				Type:  protobuf.UserSettingType_PREFERRED_NAME,
				Value: "test",
			},
		},
	}

	_, err := a.PatchUserSettings(ctx, req)

	if status.Code(err) != codes.OK {
		t.Errorf("expected OK, got %v", err)
	}

	_, err = a.PatchUserSettings(ctx, req)

	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", err)
	}
}

// now remaining tests are greeting change, playback speed change

func TestPatchUserSettings_Enrolled_ChangeGreetingValid(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), md_student_enrolled)
	req := &protobuf.PatchUserSettingsRequest{
		UserSettings: []*protobuf.UserSetting{
			{
				Type:  protobuf.UserSettingType_GREETING,
				Value: "Moin",
			},
		},
	}

	settings, err := a.PatchUserSettings(ctx, req)

	fmt.Println("settings")
	fmt.Println(settings)

	if status.Code(err) != codes.OK {
		t.Errorf("expected OK, got %v", err)
	}

	any := false
	for _, s := range settings.UserSettings {
		if s.Type == protobuf.UserSettingType_GREETING && s.Value == "Moin" {
			any = true
		}
	}
	if !any {
		t.Errorf("expected to be changed, got %v", settings.UserSettings)
	}
}
