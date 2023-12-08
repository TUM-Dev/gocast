// Package api_v2 provides API endpoints for the application.
package api_v2

import (
    "context"
    "errors"
    e "github.com/TUM-Dev/gocast/api_v2/errors"
    h "github.com/TUM-Dev/gocast/api_v2/helpers"
    s "github.com/TUM-Dev/gocast/api_v2/services"
    "github.com/TUM-Dev/gocast/api_v2/protobuf"
    "github.com/TUM-Dev/gocast/model"
    "net/http"
    "github.com/TUM-Dev/gocast/tools/pathprovider"
)


// Stream related resources are all fetched according to the same schema:
// 0. Check if request is valid
// 1. Fetch the resource from the database
// 1. Check if the user is enrolled in the course of this resource or if the course is public
// 3. Parse the resource to a protobuf representation
// 4. Return the protobuf representation


func (a *API) GetStream(ctx context.Context, req *protobuf.GetStreamRequest) (*protobuf.GetStreamResponse, error) {
    a.log.Info("GetStream")

    if req.StreamID == 0 {
        return nil, e.WithStatus(http.StatusBadRequest, errors.New("stream id must not be empty"))
    }

	uID, err := a.getCurrentID(ctx)
	if err != nil && err.Error() != "missing cookie header" {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}

	stream, err := s.GetStreamByID(a.db, uint(req.StreamID))
    if err != nil {
        return nil, err
    }

    isAllowed, err := h.CheckEnrolledOrPublic(a.db, &uID, stream.CourseID)
    if err != nil || !isAllowed {
        return nil, err
    }

    s, err := h.ParseStreamToProto(*stream)
    if err != nil {
        return nil, err
    }

    return &protobuf.GetStreamResponse{Stream: s}, nil
}

func (a *API) GetNowLive(ctx context.Context, req *protobuf.GetNowLiveRequest) (*protobuf.GetNowLiveResponse, error) {
    a.log.Info("GetNowLive")

	uID, err := a.getCurrentID(ctx)
	if err != nil && err.Error() != "missing cookie header" {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}

	
    streams, err := s.GetEnrolledOrPublicLiveStreams(a.db, &uID)
    if err != nil {
        return nil, err
    }

    resp := make([]*protobuf.Stream, len(streams))
    for i, stream := range streams {
        s, err := h.ParseStreamToProto(*stream)
        if err != nil {
            return nil, err
        } 
        resp[i] = s
    }

    return &protobuf.GetNowLiveResponse{Stream: resp}, nil
}

func (a *API) GetThumbsVOD(ctx context.Context, req *protobuf.GetThumbsVODRequest) (*protobuf.GetThumbsVODResponse, error) {
    a.log.Info("GetThumbsVOD")
    
    if req.StreamID == 0 {
        return nil, e.WithStatus(http.StatusBadRequest, errors.New("stream id must not be empty"))
    }

	uID, err := a.getCurrentID(ctx)
	if err != nil && err.Error() != "missing cookie header" {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}
	
	stream, err := s.GetStreamByID(a.db, uint(req.StreamID))
    if err != nil {
        return nil, err
    }

    isAllowed, err := h.CheckEnrolledOrPublic(a.db, &uID, stream.CourseID)
    if err != nil || !isAllowed {
        return nil, err
    }

    path, err := model.Stream.GetLGThumbnail(*stream)
    if err != nil {
        path = "/thumb-fallback.png"
    }

    return &protobuf.GetThumbsVODResponse{Path: path}, nil
}

func (a *API) GetThumbsLive(ctx context.Context, req *protobuf.GetThumbsLiveRequest) (*protobuf.GetThumbsLiveResponse, error) {
    a.log.Info("GetThumbsLive")
    
    if req.StreamID == 0 {
        return nil, e.WithStatus(http.StatusBadRequest, errors.New("stream id must not be empty"))
    }

	uID, err := a.getCurrentID(ctx)
	if err != nil && err.Error() != "missing cookie header" {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}
	
	stream, err := s.GetStreamByID(a.db, uint(req.StreamID))
    if err != nil {
        return nil, err
    }

    isAllowed, err := h.CheckEnrolledOrPublic(a.db, &uID, stream.CourseID)
    if err != nil || !isAllowed {
        return nil, err
    }

    path := pathprovider.LiveThumbnail(string(req.StreamID))
    if path == "" {
        path = "/thumb-fallback.png"
    }

    
    return &protobuf.GetThumbsLiveResponse{Path: path}, nil
}