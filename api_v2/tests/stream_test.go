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
////////////////////////////////////////////////////// GET STREAM //////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func TestGetStream(t *testing.T) {
	// Get "public" course's stream for unauthenticated users
	ctx := metadata.NewIncomingContext(context.Background(), nil)
	req := &protobuf.GetStreamRequest{StreamID: 1}
	_, err := a.GetStream(ctx, req)
	if status.Code(err) != codes.OK {
		t.Errorf("expected OK, got %v", err)
	}

	// Get "loggedin" course's stream for loggedin users
	ctx = metadata.NewIncomingContext(context.Background(), md_student_loggedin)
	req = &protobuf.GetStreamRequest{StreamID: 7}
	_, err = a.GetStream(ctx, req)
	if status.Code(err) != codes.OK {
		t.Errorf("expected OK, got %v", err)
	}

	// Get "enrolled" course's stream for enrolled users
	ctx = metadata.NewIncomingContext(context.Background(), md_student_enrolled)
	req = &protobuf.GetStreamRequest{StreamID: 8}
	_, err = a.GetStream(ctx, req)
	if status.Code(err) != codes.OK {
		t.Errorf("expected OK, got %v", err)
	}
}

func TestGetStream_InvalidArgument(t *testing.T) {
	// Call GetStream without a StreamID
	ctx := metadata.NewIncomingContext(context.Background(), md_student_loggedin)
	req := &protobuf.GetStreamRequest{}
	_, err := a.GetStream(ctx, req)
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected INVALID_ARGUMENT, got %v", err)
	}
}

func TestGetStream_Unauthenticated(t *testing.T) {
	// Call GetStream with an invalid JWT token
	ctx := metadata.NewIncomingContext(context.Background(), md_invalid_jwt)
	req := &protobuf.GetStreamRequest{StreamID: 1}
	_, err := a.GetStream(ctx, req)
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected UNAUTHENTICATED, got %v", err)
	}
}

func TestGetStream_PermissionDenied(t *testing.T) {
	// Course has visibility "loggedin" and user not loggedin
	ctx := metadata.NewIncomingContext(context.Background(), nil)
	req := &protobuf.GetStreamRequest{StreamID: 7}
	_, err := a.GetStream(ctx, req)
	if status.Code(err) != codes.PermissionDenied {
		t.Errorf("expected PERMISSION_DENIED, got %v", err)
	}

	// Course has visibility "enrolled" and user not enrolled in the course
	ctx = metadata.NewIncomingContext(context.Background(), nil)
	req = &protobuf.GetStreamRequest{StreamID: 8}
	_, err = a.GetStream(ctx, req)
	if status.Code(err) != codes.PermissionDenied {
		t.Errorf("expected PERMISSION_DENIED, got %v", err)
	}
	ctx = metadata.NewIncomingContext(context.Background(), md_student_loggedin)
	req = &protobuf.GetStreamRequest{StreamID: 8}
	_, err = a.GetStream(ctx, req)
	if status.Code(err) != codes.PermissionDenied {
		t.Errorf("expected PERMISSION_DENIED, got %v", err)
	}

	// Course has visibility "private"
	ctx = metadata.NewIncomingContext(context.Background(), nil)
	req = &protobuf.GetStreamRequest{StreamID: 9}
	_, err = a.GetStream(ctx, req)
	if status.Code(err) != codes.PermissionDenied {
		t.Errorf("expected PERMISSION_DENIED, got %v", err)
	}
	ctx = metadata.NewIncomingContext(context.Background(), md_student_loggedin)
	req = &protobuf.GetStreamRequest{StreamID: 9}
	_, err = a.GetStream(ctx, req)
	if status.Code(err) != codes.PermissionDenied {
		t.Errorf("expected PERMISSION_DENIED, got %v", err)
	}
	ctx = metadata.NewIncomingContext(context.Background(), md_student_enrolled)
	req = &protobuf.GetStreamRequest{StreamID: 9}
	_, err = a.GetStream(ctx, req)
	if status.Code(err) != codes.PermissionDenied {
		t.Errorf("expected PERMISSION_DENIED, got %v", err)
	}
}

func TestGetStream_NotFound(t *testing.T) {
	// Call GetStream with a non-existing StreamID
	ctx := metadata.NewIncomingContext(context.Background(), md_student_loggedin)
	req := &protobuf.GetStreamRequest{StreamID: 999999}
	_, err := a.GetStream(ctx, req)
	if status.Code(err) != codes.NotFound {
		t.Errorf("expected NOT_FOUND, got %v", err)
	}
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
/////////////////////////////////////////////////// GET LIVE COURSES ///////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func TestGetNowLive(t *testing.T) {
	// Get "public" live courses for unauthenticated users
	ctx := metadata.NewIncomingContext(context.Background(), nil)
	req := &protobuf.GetNowLiveRequest{}
	_, err := a.GetNowLive(ctx, req)
	if status.Code(err) != codes.OK {
		t.Errorf("expected OK, got %v", err)
	}

	// Get "loggedin" live courses for loggedin users
	req = &protobuf.GetNowLiveRequest{}
	_, err = a.GetNowLive(ctx, req)
	if status.Code(err) != codes.OK {
		t.Errorf("expected OK, got %v", err)
	}

	// Get "enrolled" live courses for enrolled users
	req = &protobuf.GetNowLiveRequest{}
	_, err = a.GetNowLive(ctx, req)
	if status.Code(err) != codes.OK {
		t.Errorf("expected OK, got %v", err)
	}
}

func TestGetNowLive_Unauthenticated(t *testing.T) {
	// Get "public" live courses with invalid jwt
	ctx := metadata.NewIncomingContext(context.Background(), md_invalid_jwt)
	req := &protobuf.GetNowLiveRequest{}
	_, err := a.GetNowLive(ctx, req)
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected UNAUTHENTICATED, got %v", err)
	}
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//////////////////////////////////////////////////// GET THUMBS VOD ////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func TestGetThumbsVOD(t *testing.T) {
	// Get "public" course's VoD thumbnail for unauthenticated users
	ctx := metadata.NewIncomingContext(context.Background(), nil)
	req := &protobuf.GetThumbsVODRequest{StreamID: 1}
	_, err := a.GetThumbsVOD(ctx, req)
	if status.Code(err) != codes.OK {
		t.Errorf("expected OK, got %v", err)
	}

	// Get "loggedin" course's VoD thumbnail for loggedin users
	ctx = metadata.NewIncomingContext(context.Background(), md_student_loggedin)
	req = &protobuf.GetThumbsVODRequest{StreamID: 7}
	_, err = a.GetThumbsVOD(ctx, req)
	if status.Code(err) != codes.OK {
		t.Errorf("expected OK, got %v", err)
	}

	// Get "enrolled" course's VoD thumbnail for enrolled users
	ctx = metadata.NewIncomingContext(context.Background(), md_student_enrolled)
	req = &protobuf.GetThumbsVODRequest{StreamID: 8}
	_, err = a.GetThumbsVOD(ctx, req)
	if status.Code(err) != codes.OK {
		t.Errorf("expected OK, got %v", err)
	}
}

func TestGetThumbsVOD_InvalidArgument(t *testing.T) {
	// Call GetThumbsVOD without a StreamID
	ctx := metadata.NewIncomingContext(context.Background(), md_student_loggedin)
	req := &protobuf.GetThumbsVODRequest{}
	_, err := a.GetThumbsVOD(ctx, req)
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected INVALID_ARGUMENT, got %v", err)
	}
}

func TestGetThumbsVOD_Unauthenticated(t *testing.T) {
	// Call GetThumbsVOD with an invalid JWT token
	ctx := metadata.NewIncomingContext(context.Background(), md_invalid_jwt)
	req := &protobuf.GetThumbsVODRequest{StreamID: 1}
	_, err := a.GetThumbsVOD(ctx, req)
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected UNAUTHENTICATED, got %v", err)
	}
}

func TestGetThumbsVOD_PermissionDenied(t *testing.T) {
	// Course has visibility "loggedin" and user not loggedin
	ctx := metadata.NewIncomingContext(context.Background(), nil)
	req := &protobuf.GetThumbsVODRequest{StreamID: 7}
	_, err := a.GetThumbsVOD(ctx, req)
	if status.Code(err) != codes.PermissionDenied {
		t.Errorf("expected PERMISSION_DENIED, got %v", err)
	}

	// Course has visibility "enrolled" and user not enrolled in the course
	ctx = metadata.NewIncomingContext(context.Background(), nil)
	req = &protobuf.GetThumbsVODRequest{StreamID: 8}
	_, err = a.GetThumbsVOD(ctx, req)
	if status.Code(err) != codes.PermissionDenied {
		t.Errorf("expected PERMISSION_DENIED, got %v", err)
	}
	ctx = metadata.NewIncomingContext(context.Background(), md_student_loggedin)
	req = &protobuf.GetThumbsVODRequest{StreamID: 8}
	_, err = a.GetThumbsVOD(ctx, req)
	if status.Code(err) != codes.PermissionDenied {
		t.Errorf("expected PERMISSION_DENIED, got %v", err)
	}

	// Course has visibility "private"
	ctx = metadata.NewIncomingContext(context.Background(), nil)
	req = &protobuf.GetThumbsVODRequest{StreamID: 9}
	_, err = a.GetThumbsVOD(ctx, req)
	if status.Code(err) != codes.PermissionDenied {
		t.Errorf("expected PERMISSION_DENIED, got %v", err)
	}
	ctx = metadata.NewIncomingContext(context.Background(), md_student_loggedin)
	req = &protobuf.GetThumbsVODRequest{StreamID: 9}
	_, err = a.GetThumbsVOD(ctx, req)
	if status.Code(err) != codes.PermissionDenied {
		t.Errorf("expected PERMISSION_DENIED, got %v", err)
	}
	ctx = metadata.NewIncomingContext(context.Background(), md_student_enrolled)
	req = &protobuf.GetThumbsVODRequest{StreamID: 9}
	_, err = a.GetThumbsVOD(ctx, req)
	if status.Code(err) != codes.PermissionDenied {
		t.Errorf("expected PERMISSION_DENIED, got %v", err)
	}
}

func TestGetThumbsVOD_NotFound(t *testing.T) {
	// Call GetThumbsVOD with a non-existing StreamID
	ctx := metadata.NewIncomingContext(context.Background(), md_student_loggedin)
	req := &protobuf.GetThumbsVODRequest{StreamID: 999999}
	_, err := a.GetThumbsVOD(ctx, req)
	if status.Code(err) != codes.NotFound {
		t.Errorf("expected NOT_FOUND, got %v", err)
	}
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
/////////////////////////////////////////////////// GET THUMBS LIVE ////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func TestGetThumbsLive(t *testing.T) {
	// Request thumbnail for currently live "public" course (unauthenticated user)
	ctx := metadata.NewIncomingContext(context.Background(), nil)
	req := &protobuf.GetThumbsLiveRequest{StreamID: 1}
	_, err := a.GetThumbsLive(ctx, req)
	if status.Code(err) != codes.OK {
		t.Errorf("expected OK, got %v", err)
	}

	// Request thumbnail for currently live "loggedin" course (loggedin user)
	ctx = metadata.NewIncomingContext(context.Background(), md_student_loggedin)
	req = &protobuf.GetThumbsLiveRequest{StreamID: 7}
	_, err = a.GetThumbsLive(ctx, req)
	if status.Code(err) != codes.OK {
		t.Errorf("expected OK, got %v", err)
	}

	// Request thumbnail for currently live "enrolled" course (enrolled user)
	ctx = metadata.NewIncomingContext(context.Background(), md_student_enrolled)
	req = &protobuf.GetThumbsLiveRequest{StreamID: 8}
	_, err = a.GetThumbsLive(ctx, req)
	if status.Code(err) != codes.OK {
		t.Errorf("expected OK, got %v", err)
	}
}

func TestGetThumbsLive_InvalidArgument(t *testing.T) {
	// Call GetThumbsLive without a StreamID
	ctx := metadata.NewIncomingContext(context.Background(), md_student_loggedin)
	req := &protobuf.GetThumbsLiveRequest{}
	_, err := a.GetThumbsLive(ctx, req)
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected INVALID_ARGUMENT, got %v", err)
	}
}

func TestGetThumbsLive_Unauthenticated(t *testing.T) {
	// Call GetThumbsLive with an invalid JWT token
	ctx := metadata.NewIncomingContext(context.Background(), md_invalid_jwt)
	req := &protobuf.GetThumbsLiveRequest{StreamID: 1}
	_, err := a.GetThumbsLive(ctx, req)
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected UNAUTHENTICATED, got %v", err)
	}
}

func TestGetThumbsLive_PermissionDenied(t *testing.T) {
	// Course has visibility "loggedin" and user not loggedin
	ctx := metadata.NewIncomingContext(context.Background(), nil)
	req := &protobuf.GetThumbsLiveRequest{StreamID: 7}
	_, err := a.GetThumbsLive(ctx, req)
	if status.Code(err) != codes.PermissionDenied {
		t.Errorf("expected PERMISSION_DENIED, got %v", err)
	}

	// Course has visibility "enrolled" and user not enrolled in the course
	ctx = metadata.NewIncomingContext(context.Background(), nil)
	req = &protobuf.GetThumbsLiveRequest{StreamID: 8}
	_, err = a.GetThumbsLive(ctx, req)
	if status.Code(err) != codes.PermissionDenied {
		t.Errorf("expected PERMISSION_DENIED, got %v", err)
	}
	ctx = metadata.NewIncomingContext(context.Background(), md_student_loggedin)
	req = &protobuf.GetThumbsLiveRequest{StreamID: 8}
	_, err = a.GetThumbsLive(ctx, req)
	if status.Code(err) != codes.PermissionDenied {
		t.Errorf("expected PERMISSION_DENIED, got %v", err)
	}

	// Course has visibility "private"
	ctx = metadata.NewIncomingContext(context.Background(), nil)
	req = &protobuf.GetThumbsLiveRequest{StreamID: 9}
	_, err = a.GetThumbsLive(ctx, req)
	if status.Code(err) != codes.PermissionDenied {
		t.Errorf("expected PERMISSION_DENIED, got %v", err)
	}
	ctx = metadata.NewIncomingContext(context.Background(), md_student_loggedin)
	req = &protobuf.GetThumbsLiveRequest{StreamID: 9}
	_, err = a.GetThumbsLive(ctx, req)
	if status.Code(err) != codes.PermissionDenied {
		t.Errorf("expected PERMISSION_DENIED, got %v", err)
	}
	ctx = metadata.NewIncomingContext(context.Background(), md_student_enrolled)
	req = &protobuf.GetThumbsLiveRequest{StreamID: 9}
	_, err = a.GetThumbsLive(ctx, req)
	if status.Code(err) != codes.PermissionDenied {
		t.Errorf("expected PERMISSION_DENIED, got %v", err)
	}
}

func TestGetThumbsLive_NotFound(t *testing.T) {
	// Call GetThumbsLive with a non-existing StreamID
	ctx := metadata.NewIncomingContext(context.Background(), md_student_loggedin)
	req := &protobuf.GetThumbsLiveRequest{StreamID: 999999}
	_, err := a.GetThumbsLive(ctx, req)
	if status.Code(err) != codes.NotFound {
		t.Errorf("expected NOT_FOUND, got %v", err)
	}
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
///////////////////////////////////////////////////// PUT PROGRESS /////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func TestPutProgress(t *testing.T) {
	// Put progress for"loggedin" course (loggedin user)
	ctx := metadata.NewIncomingContext(context.Background(), md_student_loggedin)
	req := &protobuf.PutProgressRequest{StreamID: 7, Progress: 0.5}
	_, err := a.PutProgress(ctx, req)
	if status.Code(err) != codes.OK {
		t.Errorf("expected OK, got %v", err)
	}

	// Put progress for "enrolled" course (enrolled user)
	ctx = metadata.NewIncomingContext(context.Background(), md_student_enrolled)
	req = &protobuf.PutProgressRequest{StreamID: 8, Progress: 0.5}
	_, err = a.PutProgress(ctx, req)
	if status.Code(err) != codes.OK {
		t.Errorf("expected OK, got %v", err)
	}
}

func TestPutProgress_InvalidArgument(t *testing.T) {
	// Call PutProgress without a StreamID
	ctx := metadata.NewIncomingContext(context.Background(), md_student_loggedin)
	req := &protobuf.PutProgressRequest{Progress: 0.5}
	_, err := a.PutProgress(ctx, req)
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected INVALID_ARGUMENT, got %v", err)
	}

	// Call PutProgress without setting Progress
	req = &protobuf.PutProgressRequest{StreamID: 1}
	_, err = a.PutProgress(ctx, req)
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected INVALID_ARGUMENT, got %v", err)
	}

	// Call PutProgress with invalid Progress value
	req = &protobuf.PutProgressRequest{StreamID: 1, Progress: -1}
	_, err = a.PutProgress(ctx, req)
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected INVALID_ARGUMENT, got %v", err)
	}
	req = &protobuf.PutProgressRequest{StreamID: 1, Progress: 0}
	_, err = a.PutProgress(ctx, req)
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected INVALID_ARGUMENT, got %v", err)
	}
	req = &protobuf.PutProgressRequest{StreamID: 1, Progress: 1.5}
	_, err = a.PutProgress(ctx, req)
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected INVALID_ARGUMENT, got %v", err)
	}
}

func TestPutProgress_Unauthenticated(t *testing.T) {
	// Case invalid jwt
	ctx := metadata.NewIncomingContext(context.Background(), md_invalid_jwt)
	req := &protobuf.PutProgressRequest{StreamID: 1, Progress: 0.5}
	_, err := a.PutProgress(ctx, req)
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected UNAUTHENTICATED, got %v", err)
	}
	// Case missing jwt
	ctx = metadata.NewIncomingContext(context.Background(), nil)
	req = &protobuf.PutProgressRequest{StreamID: 1, Progress: 0.5}
	_, err = a.PutProgress(ctx, req)
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected UNAUTHENTICATED, got %v", err)
	}
}

func TestPutProgress_PermissionDenied(t *testing.T) {
	// Course has visibility "enrolled" and user not enrolled in the course
	ctx := metadata.NewIncomingContext(context.Background(), md_student_loggedin)
	req := &protobuf.PutProgressRequest{StreamID: 8, Progress: 0.5}
	_, err := a.PutProgress(ctx, req)
	if status.Code(err) != codes.PermissionDenied {
		t.Errorf("expected PERMISSION_DENIED, got %v", err)
	}

	// Course has visibility "private"
	ctx = metadata.NewIncomingContext(context.Background(), md_student_loggedin)
	req = &protobuf.PutProgressRequest{StreamID: 9, Progress: 0.5}
	_, err = a.PutProgress(ctx, req)
	if status.Code(err) != codes.PermissionDenied {
		t.Errorf("expected PERMISSION_DENIED, got %v", err)
	}
	ctx = metadata.NewIncomingContext(context.Background(), md_student_enrolled)
	req = &protobuf.PutProgressRequest{StreamID: 9, Progress: 0.5}
	_, err = a.PutProgress(ctx, req)
	if status.Code(err) != codes.PermissionDenied {
		t.Errorf("expected PERMISSION_DENIED, got %v", err)
	}
}

func TestPutProgress_NotFound(t *testing.T) {
	// Call PutProgress with a non-existing StreamID
	ctx := metadata.NewIncomingContext(context.Background(), md_student_loggedin)
	req := &protobuf.PutProgressRequest{StreamID: 999999, Progress: 0.5}
	_, err := a.PutProgress(ctx, req)
	if status.Code(err) != codes.NotFound {
		t.Errorf("expected NOT_FOUND, got %v", err)
	}
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
///////////////////////////////////////////////////// GET PROGRESS /////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func TestGetProgress(t *testing.T) {
	// Get progress for"loggedin" course (loggedin user)
	ctx := metadata.NewIncomingContext(context.Background(), md_student_loggedin)
	req := &protobuf.GetProgressRequest{StreamID: 7}
	_, err := a.GetProgress(ctx, req)
	if status.Code(err) != codes.OK {
		t.Errorf("expected OK, got %v", err)
	}

	// Get progress for "enrolled" course (enrolled user)
	ctx = metadata.NewIncomingContext(context.Background(), md_student_enrolled)
	req = &protobuf.GetProgressRequest{StreamID: 8}
	_, err = a.GetProgress(ctx, req)
	if status.Code(err) != codes.OK {
		t.Errorf("expected OK, got %v", err)
	}
}

func TestGetProgress_InvalidArgument(t *testing.T) {
	// Call GetProgress without a StreamID
	ctx := metadata.NewIncomingContext(context.Background(), md_student_loggedin)
	req := &protobuf.GetProgressRequest{}
	_, err := a.GetProgress(ctx, req)
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected INVALID_ARGUMENT, got %v", err)
	}
}

func TestGetProgress_Unauthenticated(t *testing.T) {
	// Case invalid jwt
	ctx := metadata.NewIncomingContext(context.Background(), md_invalid_jwt)
	req := &protobuf.GetProgressRequest{StreamID: 1}
	_, err := a.GetProgress(ctx, req)
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected UNAUTHENTICATED, got %v", err)
	}
	// Case missing jwt
	ctx = metadata.NewIncomingContext(context.Background(), nil)
	req = &protobuf.GetProgressRequest{StreamID: 1}
	_, err = a.GetProgress(ctx, req)
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected UNAUTHENTICATED, got %v", err)
	}
}

func TestGetProgress_PermissionDenied(t *testing.T) {
	// Course has visibility "enrolled" and user not enrolled in the course
	ctx := metadata.NewIncomingContext(context.Background(), md_student_loggedin)
	req := &protobuf.GetProgressRequest{StreamID: 8}
	_, err := a.GetProgress(ctx, req)
	if status.Code(err) != codes.PermissionDenied {
		t.Errorf("expected PERMISSION_DENIED, got %v", err)
	}

	// Course has visibility "private"
	ctx = metadata.NewIncomingContext(context.Background(), md_student_loggedin)
	req = &protobuf.GetProgressRequest{StreamID: 9}
	_, err = a.GetProgress(ctx, req)
	if status.Code(err) != codes.PermissionDenied {
		t.Errorf("expected PERMISSION_DENIED, got %v", err)
	}
	ctx = metadata.NewIncomingContext(context.Background(), md_student_enrolled)
	req = &protobuf.GetProgressRequest{StreamID: 9}
	_, err = a.GetProgress(ctx, req)
	if status.Code(err) != codes.PermissionDenied {
		t.Errorf("expected PERMISSION_DENIED, got %v", err)
	}
}

func TestGetProgress_NotFound(t *testing.T) {
	// Call GetProgress with a non-existing StreamID
	ctx := metadata.NewIncomingContext(context.Background(), md_student_loggedin)
	req := &protobuf.GetProgressRequest{StreamID: 999999}
	_, err := a.GetProgress(ctx, req)
	if status.Code(err) != codes.NotFound {
		t.Errorf("expected NOT_FOUND, got %v", err)
	}
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
/////////////////////////////////////////////////// MARK AS WATCHED ////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func TestMarkAsWatched(t *testing.T) {
	// Mark "loggedin" course as watched (loggedin user)
	ctx := metadata.NewIncomingContext(context.Background(), md_student_loggedin)
	req := &protobuf.MarkAsWatchedRequest{StreamID: 7}
	_, err := a.MarkAsWatched(ctx, req)
	if status.Code(err) != codes.OK {
		t.Errorf("expected OK, got %v", err)
	}

	// Mark "enrolled" course as watched (enrolled user)
	ctx = metadata.NewIncomingContext(context.Background(), md_student_enrolled)
	req = &protobuf.MarkAsWatchedRequest{StreamID: 8}
	_, err = a.MarkAsWatched(ctx, req)
	if status.Code(err) != codes.OK {
		t.Errorf("expected OK, got %v", err)
	}
}

func TestMarkAsWatched_InvalidArgument(t *testing.T) {
	// Call MarkAsWatched without a StreamID
	ctx := metadata.NewIncomingContext(context.Background(), md_student_loggedin)
	req := &protobuf.MarkAsWatchedRequest{}
	_, err := a.MarkAsWatched(ctx, req)
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected INVALID_ARGUMENT, got %v", err)
	}
}

func TestMarkAsWatched_Unauthenticated(t *testing.T) {
	// Case invalid jwt
	ctx := metadata.NewIncomingContext(context.Background(), md_invalid_jwt)
	req := &protobuf.MarkAsWatchedRequest{StreamID: 1}
	_, err := a.MarkAsWatched(ctx, req)
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected UNAUTHENTICATED, got %v", err)
	}
	// Case missing jwt
	ctx = metadata.NewIncomingContext(context.Background(), nil)
	req = &protobuf.MarkAsWatchedRequest{StreamID: 1}
	_, err = a.MarkAsWatched(ctx, req)
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected UNAUTHENTICATED, got %v", err)
	}
}

func TestMarkAsWatched_PermissionDenied(t *testing.T) {
	// Course has visibility "enrolled" and user not enrolled in the course
	ctx := metadata.NewIncomingContext(context.Background(), md_student_loggedin)
	req := &protobuf.MarkAsWatchedRequest{StreamID: 8}
	_, err := a.MarkAsWatched(ctx, req)
	if status.Code(err) != codes.PermissionDenied {
		t.Errorf("expected PERMISSION_DENIED, got %v", err)
	}

	// Course has visibility "private"
	ctx = metadata.NewIncomingContext(context.Background(), md_student_loggedin)
	req = &protobuf.MarkAsWatchedRequest{StreamID: 9}
	_, err = a.MarkAsWatched(ctx, req)
	if status.Code(err) != codes.PermissionDenied {
		t.Errorf("expected PERMISSION_DENIED, got %v", err)
	}
	ctx = metadata.NewIncomingContext(context.Background(), md_student_enrolled)
	req = &protobuf.MarkAsWatchedRequest{StreamID: 9}
	_, err = a.MarkAsWatched(ctx, req)
	if status.Code(err) != codes.PermissionDenied {
		t.Errorf("expected PERMISSION_DENIED, got %v", err)
	}
}

func TestMarkAsWatched_NotFound(t *testing.T) {
	// Call MarkAsWatched with a non-existing StreamID
	ctx := metadata.NewIncomingContext(context.Background(), md_student_loggedin)
	req := &protobuf.MarkAsWatchedRequest{StreamID: 999999}
	_, err := a.MarkAsWatched(ctx, req)
	if status.Code(err) != codes.NotFound {
		t.Errorf("expected NOT_FOUND, got %v", err)
	}
}
