package apiv2

import (
	"errors"
	"testing"

	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"gorm.io/gorm"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
)

func TestListCourseIntegrationGrants(t *testing.T) {
	api, _ := courseAdminAPI(t, adminCourse)
	grants := mock_dao.NewMockIntegrationGrantDao(gomock.NewController(t))
	api.dao.IntegrationGrantDao = grants
	ctx := asCaller(courseLecturer)
	grants.EXPECT().GetCourseIntegrationGrants(gomock.Any(), uint(5)).Return([]model.IntegrationGrant{
		{ID: 7, Integration: model.Integration{Name: "Course portal"}},
	}, nil)
	response, err := api.ListCourseIntegrationGrants(ctx, &protobuf.ListCourseIntegrationGrantsRequest{CourseId: 5})
	if err != nil || len(response.Grants) != 1 || response.Grants[0].Id != 7 || response.Grants[0].Name != "Course portal" {
		t.Fatalf("grants: %v, %v", response, err)
	}
	grants.EXPECT().GetCourseIntegrationGrants(gomock.Any(), uint(5)).Return(nil, errors.New("read failed"))
	_, err = api.ListCourseIntegrationGrants(ctx, &protobuf.ListCourseIntegrationGrantsRequest{CourseId: 5})
	wantCode(t, err, codes.Unknown)
}

func TestRevokeCourseIntegrationGrant(t *testing.T) {
	api, m := courseAdminAPI(t, adminCourse)
	grants := mock_dao.NewMockIntegrationGrantDao(gomock.NewController(t))
	api.dao.IntegrationGrantDao = grants
	ctx := asCaller(courseLecturer)
	grants.EXPECT().RevokeIntegrationGrant(gomock.Any(), uint(7), uint(5)).Return(nil)
	m.audits.EXPECT().Create(gomock.Any()).Return(nil)
	_, err := api.RevokeCourseIntegrationGrant(ctx, &protobuf.RevokeCourseIntegrationGrantRequest{CourseId: 5, GrantId: 7})
	if err != nil {
		t.Fatal(err)
	}
	grants.EXPECT().RevokeIntegrationGrant(gomock.Any(), uint(8), uint(5)).Return(gorm.ErrRecordNotFound)
	_, err = api.RevokeCourseIntegrationGrant(ctx, &protobuf.RevokeCourseIntegrationGrantRequest{CourseId: 5, GrantId: 8})
	wantCode(t, err, codes.NotFound)
}
