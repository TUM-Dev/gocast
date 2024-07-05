package tests

import (
	"context"
	"testing"

	"github.com/TUM-Dev/gocast/api_v2/protobuf"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////// GET PUBLIC COURSES //////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func TestGetPublicCourses(t *testing.T) {
	// Get "public" courses for unauthenticated users
	ctx := metadata.NewIncomingContext(context.Background(), nil)
	req := &protobuf.GetPublicCoursesRequest{}
	_, err := a.GetPublicCourses(ctx, req)
	if status.Code(err) != codes.OK {
		t.Errorf("expected OK, got %v", err)
	}

	// Get "loggedin" courses for loggedin users
	req = &protobuf.GetPublicCoursesRequest{}
	_, err = a.GetPublicCourses(ctx, req)
	if status.Code(err) != codes.OK {
		t.Errorf("expected OK, got %v", err)
	}

	// Get "enrolled" courses for enrolled users
	req = &protobuf.GetPublicCoursesRequest{}
	_, err = a.GetPublicCourses(ctx, req)
	if status.Code(err) != codes.OK {
		t.Errorf("expected OK, got %v", err)
	}

	// Get public courses + year
	req = &protobuf.GetPublicCoursesRequest{Year: 2021}
	_, err = a.GetPublicCourses(ctx, req)
	if status.Code(err) != codes.OK {
		t.Errorf("expected OK, got %v", err)
	}
	// Get public courses + term
	req = &protobuf.GetPublicCoursesRequest{Term: "WS"}
	_, err = a.GetPublicCourses(ctx, req)
	if status.Code(err) != codes.OK {
		t.Errorf("expected OK, got %v", err)
	}
	// Get public courses limit
	req = &protobuf.GetPublicCoursesRequest{Limit: 10}
	_, err = a.GetPublicCourses(ctx, req)
	if status.Code(err) != codes.OK {
		t.Errorf("expected OK, got %v", err)
	}
	// Get public courses + skip
	req = &protobuf.GetPublicCoursesRequest{Skip: 1}
	_, err = a.GetPublicCourses(ctx, req)
	if status.Code(err) != codes.OK {
		t.Errorf("expected OK, got %v", err)
	}
	// Get public courses + year + term + limit + skip
	req = &protobuf.GetPublicCoursesRequest{Year: 2021, Term: "WS", Limit: 10, Skip: 0}
	_, err = a.GetPublicCourses(ctx, req)
	if status.Code(err) != codes.OK {
		t.Errorf("expected OK, got %v", err)
	}
}

func TestGetPublicCourses_Unauthenticated(t *testing.T) {
	// Get "public" courses with invalid jwt
	ctx := metadata.NewIncomingContext(context.Background(), md_invalid_jwt)
	req := &protobuf.GetPublicCoursesRequest{}
	_, err := a.GetPublicCourses(ctx, req)
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected UNAUTHENTICATED, got %v", err)
	}
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//////////////////////////////////////////////////// GET SEMESTERS /////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func TestGetSemesters(t *testing.T) {
	// Call GetSemesters
	req := &protobuf.GetSemestersRequest{}
	res, err := a.GetSemesters(context.Background(), req)
	if err != nil {
		t.Fatalf("could not get semesters: %v", err)
	}

	// Check the response status
	if res.Current == nil || len(res.Semesters) == 0 {
		t.Errorf("GetSemesters returned wrong data: got %v", res)
	}
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
///////////////////////////////////////////////// GET COURSES STREAMS //////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func TestGetCourseStreams(t *testing.T) {
	// Get "public" course's stream for unauthenticated users
	ctx := metadata.NewIncomingContext(context.Background(), nil)
	req := &protobuf.GetCourseStreamsRequest{CourseID: 1}
	_, err := a.GetCourseStreams(ctx, req)
	if status.Code(err) != codes.OK {
		t.Errorf("expected OK, got %v", err)
	}

	// Get "loggedin" course's stream for loggedin users
	ctx = metadata.NewIncomingContext(context.Background(), md_student_loggedin)
	req = &protobuf.GetCourseStreamsRequest{CourseID: 2}
	_, err = a.GetCourseStreams(ctx, req)
	if status.Code(err) != codes.OK {
		t.Errorf("expected OK, got %v", err)
	}

	// Get "enrolled" course's stream for enrolled users
	ctx = metadata.NewIncomingContext(context.Background(), md_student_enrolled)
	req = &protobuf.GetCourseStreamsRequest{CourseID: 3}
	_, err = a.GetCourseStreams(ctx, req)
	if status.Code(err) != codes.OK {
		t.Errorf("expected OK, got %v", err)
	}

	// Call GetCourseStreams for a course without streams
	ctx = metadata.NewIncomingContext(context.Background(), nil)
	req = &protobuf.GetCourseStreamsRequest{CourseID: 5}
	_, err = a.GetCourseStreams(ctx, req)
	if status.Code(err) != codes.OK {
		t.Errorf("expected OK, got %v", err)
	}
}

func TestGetCourseStreams_InvalidArgument(t *testing.T) {
	// Call GetCourseStreams without a CourseID
	ctx := metadata.NewIncomingContext(context.Background(), md_student_loggedin)
	req := &protobuf.GetCourseStreamsRequest{}
	_, err := a.GetCourseStreams(ctx, req)
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected INVALID_ARGUMENT, got %v", err)
	}
}

func TestGetCourseStreams_Unauthenticated(t *testing.T) {
	// Call GetCourseStreams with an invalid JWT token
	ctx := metadata.NewIncomingContext(context.Background(), md_invalid_jwt)
	req := &protobuf.GetCourseStreamsRequest{CourseID: 1}
	_, err := a.GetCourseStreams(ctx, req)
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected UNAUTHENTICATED, got %v", err)
	}
}

func TestGetCourseStreams_PermissionDenied(t *testing.T) {
	// Course has visibility "loggedin" and user not loggedin
	ctx := metadata.NewIncomingContext(context.Background(), nil)
	req := &protobuf.GetCourseStreamsRequest{CourseID: 2}
	_, err := a.GetCourseStreams(ctx, req)
	if status.Code(err) != codes.PermissionDenied {
		t.Errorf("expected PERMISSION_DENIED, got %v", err)
	}

	// Course has visibility "enrolled" and user not enrolled in the course
	ctx = metadata.NewIncomingContext(context.Background(), nil)
	req = &protobuf.GetCourseStreamsRequest{CourseID: 3}
	_, err = a.GetCourseStreams(ctx, req)
	if status.Code(err) != codes.PermissionDenied {
		t.Errorf("expected PERMISSION_DENIED, got %v", err)
	}
	ctx = metadata.NewIncomingContext(context.Background(), md_student_loggedin)
	_, err = a.GetCourseStreams(ctx, req)
	if status.Code(err) != codes.PermissionDenied {
		t.Errorf("expected PERMISSION_DENIED, got %v", err)
	}

	// Course has visibility "private"
	ctx = metadata.NewIncomingContext(context.Background(), nil)
	req = &protobuf.GetCourseStreamsRequest{CourseID: 4}
	_, err = a.GetCourseStreams(ctx, req)
	if status.Code(err) != codes.PermissionDenied {
		t.Errorf("expected PERMISSION_DENIED, got %v", err)
	}
	ctx = metadata.NewIncomingContext(context.Background(), md_student_loggedin)
	_, err = a.GetCourseStreams(ctx, req)
	if status.Code(err) != codes.PermissionDenied {
		t.Errorf("expected PERMISSION_DENIED, got %v", err)
	}
	ctx = metadata.NewIncomingContext(context.Background(), md_student_enrolled)
	_, err = a.GetCourseStreams(ctx, req)
	if status.Code(err) != codes.PermissionDenied {
		t.Errorf("expected PERMISSION_DENIED, got %v", err)
	}
}

func TestGetCourseStreams_NotFound(t *testing.T) {
	// Call GetCourseStreams with a non-existing CourseID
	ctx := metadata.NewIncomingContext(context.Background(), md_student_loggedin)
	req := &protobuf.GetCourseStreamsRequest{CourseID: 999999}
	_, err := a.GetCourseStreams(ctx, req)
	if status.Code(err) != codes.NotFound {
		t.Errorf("expected NOT_FOUND, got %v", err)
	}
}
