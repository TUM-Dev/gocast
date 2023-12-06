// Package api_v2 provides API endpoints for the application.
package api_v2

import (
	"context"
	"net/http"

	e "github.com/TUM-Dev/gocast/api_v2/errors"
	h "github.com/TUM-Dev/gocast/api_v2/helpers"
	"github.com/TUM-Dev/gocast/api_v2/protobuf"
	s "github.com/TUM-Dev/gocast/api_v2/services"
)

// GetUser retrieves the current user based on the context.
// It returns a GetUserResponse or an error if one occurs.
func (a *API) GetUser(ctx context.Context, req *protobuf.GetUserRequest) (*protobuf.GetUserResponse, error) {
	a.log.Info("GetUser")
	u, err := a.getCurrent(ctx)
	if err != nil {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}

	response := &protobuf.GetUserResponse{
		User: h.ParseUserToProto(u),
	}

	return response, nil
}

// GetUserCourses retrieves the courses of a user based on the context and request.
// It filters the courses by year, term, query, limit, and skip if they are specified in the request.
// It returns a GetUserCoursesResponse or an error if one occurs.
func (a *API) GetUserCourses(ctx context.Context, req *protobuf.GetUserCoursesRequest) (*protobuf.GetUserCoursesResponse, error) {
	a.log.Info("GetUserCourses")
	u, err := a.getCurrent(ctx)
	if err != nil {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}

	courses, err := s.FetchUserCourses(a.db, u.ID, req)
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	resp := make([]*protobuf.Course, len(courses))

	for i, course := range courses {
		resp[i] = h.ParseCourseToProto(course)
	}

	return &protobuf.GetUserCoursesResponse{
		Courses: resp,
	}, nil
}

// GetUserPinned retrieves the pinned courses of a user based on the context and request.
// It filters the courses by year, term, limit, and skip if they are specified in the request.
// It returns a GetUserPinnedResponse or an error if one occurs.
func (a *API) GetUserPinned(ctx context.Context, req *protobuf.GetUserPinnedRequest) (*protobuf.GetUserPinnedResponse, error) {
	a.log.Info("GetUserPinned")
	u, err := a.getCurrent(ctx)
	if err != nil {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}

	courses, err := s.FetchUserPinnedCourses(a.db, *u, req)
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	resp := make([]*protobuf.Course, len(courses))

	for i, course := range courses {
		resp[i] = h.ParseCourseToProto(course)
	}

	return &protobuf.GetUserPinnedResponse{
		Courses: resp,
	}, nil
}

// GetUserAdminCourses retrieves the courses of a user in which he is an admin based on the context.
// It returns a GetUserAdminResponse or an error if one occurs.
func (a *API) GetUserAdminCourses(ctx context.Context, req *protobuf.GetUserAdminRequest) (*protobuf.GetUserAdminResponse, error) {
	a.log.Info("GetUserAdminCourses")
	u, err := a.getCurrent(ctx)
	if err != nil {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}

	courses, err := s.FetchUserAdminCourses(a.db, u.ID)
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	resp := make([]*protobuf.Course, len(courses))

	for i, course := range courses {
		resp[i] = h.ParseCourseToProto(course)
	}

	return &protobuf.GetUserAdminResponse{
		Courses: resp,
	}, nil
}

// GetUserBookmarks retrieves the bookmarks of a user based on the context and request.
// It filters the bookmarks by stream ID if it is specified in the request.
// It returns a GetBookmarksResponse or an error if one occurs.
func (a *API) GetUserBookmarks(ctx context.Context, req *protobuf.GetBookmarksRequest) (*protobuf.GetBookmarksResponse, error) {
	a.log.Info("GetUserBookmarks")
	u, err := a.getCurrent(ctx)
	if err != nil {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}

	bookmarks, err := s.FetchUserBookmarks(a.db, u.ID, req)

	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	resp := make([]*protobuf.Bookmark, len(bookmarks))

	for i, bookmark := range bookmarks {
		resp[i] = h.ParseBookmarkToProto(bookmark)
	}

	return &protobuf.GetBookmarksResponse{
		Bookmarks: resp,
	}, nil
}

// PutUserBookmark put bookmark
func (a *API) PutUserBookmark(ctx context.Context, req *protobuf.PutBookmarkRequest) (*protobuf.PutBookmarkResponse, error) {
	a.log.Info("PutUserBookmark")
	u, err := a.getCurrent(ctx)
	if err != nil {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}
	bookmark, err := s.PutUserBookmark(a.db, u.ID, req)
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	return &protobuf.PutBookmarkResponse{
		Bookmark: h.ParseBookmarkToProto(bookmark),
	}, nil

}

func (a *API) PatchUserBookmark(ctx context.Context, req *protobuf.PatchBookmarkRequest) (*protobuf.PatchBookmarkResponse, error) {
	a.log.Info("PatchUserBookmark")
	u, err := a.getCurrent(ctx)

	if err != nil {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}

	bookmark, err := s.PatchUserBookmark(a.db, u.ID, req)

	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	return &protobuf.PatchBookmarkResponse{
		Bookmark: h.ParseBookmarkToProto(bookmark),
	}, nil
}

func (a *API) DeleteUserBookmark(ctx context.Context, req *protobuf.DeleteBookmarkRequest) (*protobuf.DeleteBookmarkResponse, error) {
	a.log.Info("DeleteUserBookmark")
	u, err := a.getCurrent(ctx)

	if err != nil {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}

	err = s.DeleteUserBookmark(a.db, u.ID, req)

	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	return &protobuf.DeleteBookmarkResponse{}, nil
}

func (a *API) GetBannerAlerts(ctx context.Context, req *protobuf.GetBannerAlertsRequest) (*protobuf.GetBannerAlertsResponse, error) {
	a.log.Info("GetBannerAlerts")

	alerts, err := s.FetchBannerAlerts(a.db)

	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	resp := make([]*protobuf.BannerAlert, len(alerts))

	for i, alert := range alerts {
		resp[i] = h.ParseBannerAlertToProto(alert)
	}

	return &protobuf.GetBannerAlertsResponse{
		BannerAlerts: resp,
	}, nil

}

func (a *API) GetFeatureNotifications(ctx context.Context, req *protobuf.GetFeatureNotificationsRequest) (*protobuf.GetFeatureNotificationsResponse, error) {
	a.log.Info("GetUserNotifications")
	u, err := a.getCurrent(ctx)

	if err != nil {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}

	notifications, err := s.FetchUserNotifications(a.db, u)

	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	resp := make([]*protobuf.FeatureNotification, len(notifications))

	for i, notification := range notifications {
		resp[i] = h.ParseFeatureNotificationToProto(notification)
	}

	return &protobuf.GetFeatureNotificationsResponse{
		FeatureNotifications: resp,
	}, nil
}

// GetUserSettings retrieves the settings of a user based on the context.
// It returns a GetUserSettingsResponse or an error if one occurs.
func (a *API) GetUserSettings(ctx context.Context, req *protobuf.GetUserSettingsRequest) (*protobuf.GetUserSettingsResponse, error) {
	a.log.Info("GetUserSettings")
	u, err := a.getCurrent(ctx)
	if err != nil {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}

	settings, err := s.FetchUserSettings(a.db, u.ID)
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	resp := make([]*protobuf.UserSetting, len(settings))

	for i, setting := range settings {
		resp[i] = h.ParseUserSettingToProto(setting)
	}

	return &protobuf.GetUserSettingsResponse{
		UserSettings: resp,
	}, nil
}
