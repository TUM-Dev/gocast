package api_v2

import (
	"context"

	"github.com/TUM-Dev/gocast/api_v2/protobuf"
	"github.com/TUM-Dev/gocast/tools/tum"
	"google.golang.org/protobuf/types/known/emptypb"
)

// GetSemesters retrieves all available semesters and the current semester.
func (a *API) GetSemesters(ctx context.Context, req *emptypb.Empty) (*protobuf.GetSemestersResponse, error) {
	semesters := a.dao.GetAvailableSemesters(ctx)
	year, term := tum.GetCurrentSemester()

	resp := &protobuf.GetSemestersResponse{
		Current: &protobuf.Semester{
			Year:         uint32(year),
			TeachingTerm: term,
		},
		Semesters: make([]*protobuf.Semester, len(semesters)),
	}

	for i, semester := range semesters {
		resp.Semesters[i] = &protobuf.Semester{
			Year:         uint32(semester.Year),
			TeachingTerm: semester.TeachingTerm,
		}
	}

	return resp, nil
}
