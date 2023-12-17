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


func (a *API) handleStreamRequest(ctx context.Context, sID uint64) (*model.Stream, error) {
    if sID == 0 {
        return nil, e.WithStatus(http.StatusBadRequest, errors.New("stream id must not be empty"))
    }

    uID, err := a.getCurrentID(ctx)
	if err != nil && err.Error() != "missing cookie header" {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}

	s, err := s.GetStreamByID(a.db, uint(sID))
    if err != nil {
        return nil, err
    }

    c, err := h.CheckAuthorized(a.db, uID, s.CourseID)
    if err != nil {
        return nil, err
    }

    if err = h.SignStream(s, c, uID); err != nil {
        return nil, err
    }

    return s, nil
}

func (a *API) GetStream(ctx context.Context, req *protobuf.GetStreamRequest) (*protobuf.GetStreamResponse, error) {
    a.log.Info("GetStream")

    s, err := a.handleStreamRequest(ctx, req.StreamID)
    if err != nil {
        return nil, err
    }

    stream, err := h.ParseStreamToProto(s)
    if err != nil {
        return nil, err
    }

    return &protobuf.GetStreamResponse{Stream: stream}, nil
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

        c, err := s.GetCourseById(a.db, stream.CourseID)
        if err != nil {
            return nil, err
        }
                
        if err := h.SignStream(stream, c, uID); err != nil {
            return nil, err
        }

        s, err := h.ParseStreamToProto(stream)
        if err != nil {
            return nil, err
        } 
        resp[i] = s

    }

    return &protobuf.GetNowLiveResponse{Stream: resp}, nil
}

func (a *API) GetThumbsVOD(ctx context.Context, req *protobuf.GetThumbsVODRequest) (*protobuf.GetThumbsVODResponse, error) {
    a.log.Info("GetThumbsVOD")
    
    s, err := a.handleStreamRequest(ctx, req.StreamID)
    if err != nil {
        return nil, err
    }

    path, err := model.Stream.GetLGThumbnail(*s)
    if err != nil {
        path = "/thumb-fallback.png"
    }

    return &protobuf.GetThumbsVODResponse{Path: path}, nil
}

func (a *API) GetThumbsLive(ctx context.Context, req *protobuf.GetThumbsLiveRequest) (*protobuf.GetThumbsLiveResponse, error) {
    a.log.Info("GetThumbsLive")
    
    s, err := a.handleStreamRequest(ctx, req.StreamID)
    if err != nil {
        return nil, err
    }

    path := pathprovider.LiveThumbnail(string(s.ID))
    if path == "" {
        path = "/thumb-fallback.png"
    }

    
    return &protobuf.GetThumbsLiveResponse{Path: path}, nil
}