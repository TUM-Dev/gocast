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
////////////////////////////////////////////////// GET BANNER ALERTS ///////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func TestGetBannerAlerts(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), nil)
	req := &protobuf.GetBannerAlertsRequest{}
	_, err := a.GetBannerAlerts(ctx, req)
	if status.Code(err) != codes.OK {
		t.Errorf("expected OK, got %v", err)
	}
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////// GET FEATURE NOTIFICATIONS ///////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func TestGetFeatureNotifications(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), md_student_loggedin)
	req := &protobuf.GetFeatureNotificationsRequest{}
	_, err := a.GetFeatureNotifications(ctx, req)
	if status.Code(err) != codes.OK {
		t.Errorf("expected OK, got %v", err)
	}
}

func TestGetFeatureNotifications_Unauthenticated(t *testing.T) {
	// Case invalid jwt
	ctx := metadata.NewIncomingContext(context.Background(), md_invalid_jwt)
	req := &protobuf.GetFeatureNotificationsRequest{}
	_, err := a.GetFeatureNotifications(ctx, req)
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected UNAUTHENTdeviceTokenICATED, got %v", err)
	}

	// Case missing jwt
	ctx = metadata.NewIncomingContext(context.Background(), nil)
	req = &protobuf.GetFeatureNotificationsRequest{}
	_, err = a.GetFeatureNotifications(ctx, req)
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected UNAUTHENTICATED, got %v", err)
	}
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////// POST DEVICE TOKEN ///////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func TestPostDeviceToken(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), md_student_loggedin)
	req := &protobuf.PostDeviceTokenRequest{DeviceToken: "TestPostDeviceToken"}
	_, err := a.PostDeviceToken(ctx, req)
	if status.Code(err) != codes.OK {
		t.Errorf("expected OK, got %v", err)
	}
}

func TestPostDeviceToken_InvalidArgument(t *testing.T) {
	// Case device_token blank
	ctx := metadata.NewIncomingContext(context.Background(), md_invalid_jwt)
	req := &protobuf.PostDeviceTokenRequest{DeviceToken: ""}
	_, err := a.PostDeviceToken(ctx, req)
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected INVALID_ARGUMENT, got %v", err)
	}

	// Case device_token nil
	ctx = metadata.NewIncomingContext(context.Background(), md_invalid_jwt)
	req = &protobuf.PostDeviceTokenRequest{}
	_, err = a.PostDeviceToken(ctx, req)
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected INVALID_ARGUMENT, got %v", err)
	}
}

func TestPostDeviceToken_Unauthenticated(t *testing.T) {
	// Case invalid jwt
	ctx := metadata.NewIncomingContext(context.Background(), md_invalid_jwt)
	req := &protobuf.PostDeviceTokenRequest{DeviceToken: "TestPostDeviceToken_Unauthenticated"}
	_, err := a.PostDeviceToken(ctx, req)
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected UNAUTHENTICATED, got %v", err)
	}

	// Case missing jwt
	ctx = metadata.NewIncomingContext(context.Background(), nil)
	req = &protobuf.PostDeviceTokenRequest{DeviceToken: "TestPostDeviceToken_Unauthenticated"}
	_, err = a.PostDeviceToken(ctx, req)
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected UNAUTHENTICATED, got %v", err)
	}
}

func TestPostDeviceToken_AlreadyExists(t *testing.T) {
	// Case device_token already exsits (e.g., submitted same token twice)
	ctx := metadata.NewIncomingContext(context.Background(), md_student_loggedin)
	req := &protobuf.PostDeviceTokenRequest{DeviceToken: "TestPostDeviceToken_AlreadyExists"}
	_, err := a.PostDeviceToken(ctx, req)
	_, err = a.PostDeviceToken(ctx, req)
	if status.Code(err) != codes.AlreadyExists {
		t.Errorf("expected ALREAD_EXISTS, got %v", err)
	}
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
///////////////////////////////////////////////// DELETE DEVICE TOKEN //////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func TestDeleteDeviceToken(t *testing.T) {
	// Create device_token first
	ctx := metadata.NewIncomingContext(context.Background(), md_student_loggedin)
	req_post := &protobuf.PostDeviceTokenRequest{DeviceToken: "TestDeleteDeviceToken"}
	_, err := a.PostDeviceToken(ctx, req_post)

	// Delete device_token
	req := &protobuf.DeleteDeviceTokenRequest{DeviceToken: "TestDeleteDeviceToken"}
	_, err = a.DeleteDeviceToken(ctx, req)
	if status.Code(err) != codes.OK {
		t.Errorf("expected OK, got %v", err)
	}
}

func TestDeleteDeviceToken_NotFound(t *testing.T) {
	// Delete non-existing device_token
	ctx := metadata.NewIncomingContext(context.Background(), md_student_loggedin)
	req := &protobuf.DeleteDeviceTokenRequest{DeviceToken: "TestDeleteDeviceToken_NotFound"}
	_, err := a.DeleteDeviceToken(ctx, req)
	if status.Code(err) != codes.NotFound {
		t.Errorf("expected NOT_FOUND, got %v", err)
	}
}

func TestDeleteDeviceToken_InvalidArgument(t *testing.T) {
	// Case device_token blank
	ctx := metadata.NewIncomingContext(context.Background(), md_student_loggedin)
	req := &protobuf.DeleteDeviceTokenRequest{DeviceToken: ""}
	_, err := a.DeleteDeviceToken(ctx, req)
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected INVALID_ARGUMENT, got %v", err)
	}

	// Case device_token nil
	req = &protobuf.DeleteDeviceTokenRequest{}
	_, err = a.DeleteDeviceToken(ctx, req)
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected INVALID_ARGUMENT, got %v", err)
	}
}

func TestDeleteDeviceToken_Unauthenticated(t *testing.T) {
	// Create device_token first
	ctx := metadata.NewIncomingContext(context.Background(), md_student_loggedin)
	req_post := &protobuf.PostDeviceTokenRequest{DeviceToken: "TestDeleteDeviceToken_Unauthenticated"}
	_, err := a.PostDeviceToken(ctx, req_post)

	// Case invalid jwt
	ctx = metadata.NewIncomingContext(context.Background(), md_invalid_jwt)
	req := &protobuf.DeleteDeviceTokenRequest{DeviceToken: "TestDeleteDeviceToken_Unauthenticated"}
	_, err = a.DeleteDeviceToken(ctx, req)
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected UNAUTHENTICATED, got %v", err)
	}

	// Case missing jwt
	ctx = metadata.NewIncomingContext(context.Background(), nil)
	req = &protobuf.DeleteDeviceTokenRequest{DeviceToken: "TestDeleteDeviceToken_Unauthenticated"}
	_, err = a.DeleteDeviceToken(ctx, req)
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected UNAUTHENTICATED, got %v", err)
	}
}
