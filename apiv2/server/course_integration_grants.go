package apiv2

import (
	"context"
	"fmt"
	"net/http"

	"google.golang.org/protobuf/types/known/emptypb"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/model"
)

func (a *API) ListCourseIntegrationGrants(ctx context.Context, req *protobuf.ListCourseIntegrationGrantsRequest) (*protobuf.ListCourseIntegrationGrantsResponse, error) {
	if err := refuseCourseZero(req.GetCourseId()); err != nil {
		return nil, err
	}
	grants, err := a.dao.IntegrationGrantDao.GetCourseIntegrationGrants(ctx, uint(req.GetCourseId()))
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	response := &protobuf.ListCourseIntegrationGrantsResponse{}
	for _, grant := range grants {
		response.Grants = append(response.Grants, &protobuf.CourseIntegrationGrant{Id: uint32(grant.ID), Name: grant.Integration.Name})
	}
	return response, nil
}

func (a *API) RevokeCourseIntegrationGrant(ctx context.Context, req *protobuf.RevokeCourseIntegrationGrantRequest) (*emptypb.Empty, error) {
	course, err := a.administeredCourse(ctx, req.GetCourseId())
	if err != nil {
		return nil, err
	}
	if err := a.dao.IntegrationGrantDao.RevokeIntegrationGrant(ctx, uint(req.GetGrantId()), course.ID); err != nil {
		return nil, e.FromGorm(err, "no active integration grant for this course")
	}
	a.audit(ctx, model.AuditCourseEdit, fmt.Sprintf("%s:'%s' revoke integration grant: %d", course.Name, course.Slug, req.GetGrantId()))
	return &emptypb.Empty{}, nil
}
