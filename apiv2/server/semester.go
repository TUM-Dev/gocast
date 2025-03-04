package apiv2

import (
	"context"

	h "github.com/TUM-Dev/gocast/apiv2/helpers"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/tools/tum"
	"google.golang.org/protobuf/types/known/emptypb"
)

// GetSemesters retrieves all available semesters and the current semester.
func (a *API) GetSemesters(ctx context.Context, req *emptypb.Empty) (*protobuf.GetSemestersResponse, error) {
	a.log.Info("GetSemesters")

	semesters := a.dao.GetAvailableSemesters(ctx, false)
	year, term := tum.GetCurrentSemester()

	resp := &protobuf.GetSemestersResponse{
		Current: &protobuf.Semester{
			Year:         uint32(year),
			TeachingTerm: term,
		},
		Semesters: make([]*protobuf.Semester, len(semesters)),
	}

	for i, semester := range semesters {
		resp.Semesters[i] = h.ParseSemesterToProto(semester)
	}

	return resp, nil
}
