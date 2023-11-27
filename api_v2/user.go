package api_v2

import (
	"context"
	e "github.com/TUM-Dev/gocast/api_v2/errors"
	h "github.com/TUM-Dev/gocast/api_v2/helpers"
	s "github.com/TUM-Dev/gocast/api_v2/services"
	"github.com/TUM-Dev/gocast/api_v2/protobuf"
	"net/http"
	)	

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

func (a *API) GetUserCourses(ctx context.Context, req *protobuf.GetUserCoursesRequest) (*protobuf.GetUserCoursesResponse, error) {
    a.log.Info("GetUserCourses")
    u, err := a.getCurrent(ctx)
    if err != nil {
        return nil, e.WithStatus(http.StatusUnauthorized, err)
    }

    courses, err := s.RetrieveCourses(a.db, u.ID, req)
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

func (a *API) GetUserPinned(ctx context.Context, req *protobuf.GetUserPinnedRequest) (*protobuf.GetUserPinnedResponse, error) {
    a.log.Info("GetUserPinned")
    u, err := a.getCurrent(ctx)
    if err != nil {
        return nil, e.WithStatus(http.StatusUnauthorized, err)
    }

    courses, err := s.RetrievePinnedCourses(a.db, *u, req)
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

func (a *API) GetUserAdminCourses(ctx context.Context, req *protobuf.GetUserAdminRequest) (*protobuf.GetUserAdminResponse, error) {
	a.log.Info("GetUserAdminCourses")
	u, err := a.getCurrent(ctx)
	if err != nil {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}
	
    courses, err := s.RetrieveUserAdminCourses(a.db, u.ID)
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

func (a *API) GetUserBookmarks(ctx context.Context, req *protobuf.GetBookmarksRequest) (*protobuf.GetBookmarksResponse, error) {
	a.log.Info("GetUserBookmarks")
	u, err := a.getCurrent(ctx)
	if err != nil {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}
	
    bookmarks, err := s.RetrieveBookmarks(a.db, u.ID, req)
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

func (a *API) GetUserSettings(ctx context.Context, req *protobuf.GetUserSettingsRequest) (*protobuf.GetUserSettingsResponse, error) {
	a.log.Info("GetUserSettings")
	u, err := a.getCurrent(ctx)
	if err != nil {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}
	
    settings, err := s.RetrieveUserSettings(a.db, u.ID)
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
