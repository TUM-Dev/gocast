// Package api_v2 provides API endpoints for the application.
package api_v2

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	e "github.com/TUM-Dev/gocast/api_v2/errors"
	h "github.com/TUM-Dev/gocast/api_v2/helpers"
	"github.com/TUM-Dev/gocast/api_v2/protobuf"
	s "github.com/TUM-Dev/gocast/api_v2/services"
	"github.com/TUM-Dev/gocast/model"
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

	path := pathprovider.LiveThumbnail(fmt.Sprintf("%d", s.ID))
	if path == "" {
		path = "/thumb-fallback.png"
	}

	return &protobuf.GetThumbsLiveResponse{Path: path}, nil
}

func (a *API) GetProgress(ctx context.Context, req *protobuf.GetProgressRequest) (*protobuf.GetProgressResponse, error) {
	a.log.Info("GetStreamProgress")

	uID, err := a.getCurrentID(ctx)
	if err != nil {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}

	p, err := s.GetProgress(a.db, uint(req.StreamID), uID)
	if err != nil {
		return nil, err
	}

	progress := h.ParseProgressToProto(p)

	return &protobuf.GetProgressResponse{Progress: progress}, nil
}

func (a *API) PutProgress(ctx context.Context, req *protobuf.PutProgressRequest) (*protobuf.PutProgressResponse, error) {
	a.log.Info("SetStreamProgress")

	uID, err := a.getCurrentID(ctx)
	if err != nil {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}

	p, err := s.SetProgress(a.db, uint(req.StreamID), uID, float64(req.Progress))
	if err != nil {
		return nil, err
	}

	progress := h.ParseProgressToProto(p)
	return &protobuf.PutProgressResponse{Progress: progress}, nil
}

func (a *API) MarkAsWatched(ctx context.Context, req *protobuf.MarkAsWatchedRequest) (*protobuf.MarkAsWatchedResponse, error) {
	a.log.Info("MarkAsWatched")

	uID, err := a.getCurrentID(ctx)
	if err != nil {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}

	p, err := s.MarkAsWatched(a.db, uint(req.StreamID), uID)
	if err != nil {
		return nil, err
	}

	progress := h.ParseProgressToProto(p)
	return &protobuf.MarkAsWatchedResponse{Progress: progress}, nil
}

// chat endpoints

func (a *API) GetChatMessages(ctx context.Context, req *protobuf.GetChatMessagesRequest) (*protobuf.GetChatMessagesResponse, error) {
	a.log.Info("GetChatMessages")

	uID, err := a.getCurrentID(ctx)
	if err != nil {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}

	if req.StreamID == 0 {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("stream id must not be empty"))
	}

	stream, err := s.GetStreamByID(a.db, uint(req.StreamID))
	if err != nil {
		return nil, err
	}

	_, err = h.CheckAuthorized(a.db, uID, stream.CourseID)
	if err != nil {
		return nil, err
	}

	_, err = h.CheckCanChat(a.db, uID, uint(req.StreamID))
	if err != nil {
		return nil, err
	}

	chats, err := s.GetChatMessages(a.db, uint(req.StreamID))
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	var chatMessages []*protobuf.ChatMessage

	for _, chat := range chats {
		chatMessages = append(chatMessages, h.ParseChatMessageToProto(*chat))
	}

	return &protobuf.GetChatMessagesResponse{Messages: chatMessages}, nil
}

func (a *API) PostChatMessage(ctx context.Context, req *protobuf.PostChatMessageRequest) (*protobuf.PostChatMessageResponse, error) {
	a.log.Info("PostChatMessage")

	uID, err := a.getCurrentID(ctx)
	if err != nil {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}

	if req.StreamID == 0 {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("stream id must not be empty"))
	}

	stream, err := s.GetStreamByID(a.db, uint(req.StreamID))
	if err != nil {
		return nil, err
	}

	_, err = h.CheckAuthorized(a.db, uID, stream.CourseID)
	if err != nil {
		return nil, err
	}

	_, err = h.CheckCanChat(a.db, uID, uint(req.StreamID))
	if err != nil {
		return nil, err
	}

	chat, err := s.PostChatMessage(a.db, uint(req.StreamID), uID, req.Message)
	if err != nil {
		return nil, err
	}

	chatMessage := h.ParseChatMessageToProto(*chat)

	return &protobuf.PostChatMessageResponse{Message: chatMessage}, nil
}

func (a *API) PostChatReaction(ctx context.Context, req *protobuf.PostChatReactionRequest) (*protobuf.PostChatReactionResponse, error) {
	a.log.Info("PostChatReaction")

	uID, err := a.getCurrentID(ctx)
	if err != nil {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}

	if req.StreamID == 0 {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("stream id must not be empty"))
	}

	stream, err := s.GetStreamByID(a.db, uint(req.StreamID))
	if err != nil {
		return nil, err
	}

	_, err = h.CheckAuthorized(a.db, uID, stream.CourseID)
	if err != nil {
		return nil, err
	}

	_, err = h.CheckCanChat(a.db, uID, uint(req.StreamID))
	if err != nil {
		return nil, err
	}

	chatReaction, err := s.PostChatReaction(a.db, uint(req.StreamID), uID, uint(req.ChatID), req.Emoji)
	if err != nil {
		return nil, err
	}

	reaction := h.ParseChatReactionToProto(*chatReaction)

	return &protobuf.PostChatReactionResponse{Reaction: reaction}, nil
}

func (a *API) DeleteChatReaction(ctx context.Context, req *protobuf.DeleteChatReactionRequest) (*protobuf.DeleteChatReactionResponse, error) {
	a.log.Info("DeleteChatReaction")

	uID, err := a.getCurrentID(ctx)
	if err != nil {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}

	if req.StreamID == 0 {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("stream id must not be empty"))
	}

	stream, err := s.GetStreamByID(a.db, uint(req.StreamID))
	if err != nil {
		return nil, err
	}

	_, err = h.CheckAuthorized(a.db, uID, stream.CourseID)
	if err != nil {
		return nil, err
	}

	_, err = h.CheckCanChat(a.db, uID, uint(req.StreamID))
	if err != nil {
		return nil, err
	}

	_, err = s.DeleteChatReaction(a.db, uint(req.StreamID), uID, uint(req.ChatID))
	if err != nil {
		return nil, err
	}

	return &protobuf.DeleteChatReactionResponse{}, nil
}

func (a *API) PostChatReply(ctx context.Context, req *protobuf.PostChatReplyRequest) (*protobuf.PostChatReplyResponse, error) {
	a.log.Info("PostChatReply")

	uID, err := a.getCurrentID(ctx)
	if err != nil {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}

	if req.StreamID == 0 {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("stream id must not be empty"))
	}

	stream, err := s.GetStreamByID(a.db, uint(req.StreamID))
	if err != nil {
		return nil, err
	}

	_, err = h.CheckAuthorized(a.db, uID, stream.CourseID)

	if err != nil {
		return nil, err
	}

	_, err = h.CheckCanChat(a.db, uID, uint(req.StreamID))
	if err != nil {
		return nil, err
	}

	chat, err := s.PostChatReply(a.db, uint(req.StreamID), uID, uint(req.ChatID), req.Message)
	if err != nil {
		return nil, err
	}

	chatMessage := h.ParseChatMessageToProto(*chat)

	return &protobuf.PostChatReplyResponse{Reply: chatMessage}, nil
}

func (a *API) MarkChatMessageAsResolved(ctx context.Context, req *protobuf.MarkChatMessageAsResolvedRequest) (*protobuf.MarkChatMessageAsResolvedResponse, error) {
	a.log.Info("MarkChatMessageAsResolved")

	uID, err := a.getCurrentID(ctx)
	if err != nil {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}

	if req.StreamID == 0 {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("stream id must not be empty"))
	}

	stream, err := s.GetStreamByID(a.db, uint(req.StreamID))
	if err != nil {
		return nil, err
	}

	_, err = h.CheckAuthorized(a.db, uID, stream.CourseID)

	if err != nil {
		return nil, err
	}

	_, err = h.CheckCanChat(a.db, uID, uint(req.StreamID))
	if err != nil {
		return nil, err
	}

	chat, err := s.MarkChatMessageAsResolved(a.db, uID, uint(req.ChatID))
	if err != nil {
		return nil, err
	}

	chatMessage := h.ParseChatMessageToProto(*chat)

	return &protobuf.MarkChatMessageAsResolvedResponse{Message: chatMessage}, nil
}

func (a *API) MarkChatMessageAsUnresolved(ctx context.Context, req *protobuf.MarkChatMessageAsUnresolvedRequest) (*protobuf.MarkChatMessageAsUnresolvedResponse, error) {
	a.log.Info("MarkChatMessageAsUnresolved")

	uID, err := a.getCurrentID(ctx)
	if err != nil {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}

	if req.StreamID == 0 {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("stream id must not be empty"))
	}

	stream, err := s.GetStreamByID(a.db, uint(req.StreamID))
	if err != nil {
		return nil, err
	}

	_, err = h.CheckAuthorized(a.db, uID, stream.CourseID)

	if err != nil {
		return nil, err
	}

	_, err = h.CheckCanChat(a.db, uID, uint(req.StreamID))
	if err != nil {
		return nil, err
	}

	chat, err := s.MarkChatMessageAsUnresolved(a.db, uID, uint(req.ChatID))
	if err != nil {
		return nil, err
	}

	chatMessage := h.ParseChatMessageToProto(*chat)

	return &protobuf.MarkChatMessageAsUnresolvedResponse{Message: chatMessage}, nil
}
