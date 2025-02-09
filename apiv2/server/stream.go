package apiv2

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strconv"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	h "github.com/TUM-Dev/gocast/apiv2/helpers"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools/pathprovider"
	"google.golang.org/genproto/googleapis/api/httpbody"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"
)

// GetStream returns a stream by its ID including the course and lecture hall
func (a *API) GetStream(ctx context.Context, req *protobuf.GetStreamRequest) (*protobuf.CourseStream, error) {
	a.log.Info("GetStream")

	_, stream, course, err := a.authorizeUserForStreamCourse(ctx, req)
	if err != nil {
		return nil, err
	}

	lectureHall := &model.LectureHall{}
	if stream.LectureHallID != 0 {
		lh, err := a.dao.LectureHallsDao.GetLectureHallByID(stream.LectureHallID)
		if err != nil {
			a.log.Error("Could not get Lecture Hall ID", "err", err)
		} else {
			lectureHall = &lh
		}
	}

	return &protobuf.CourseStream{
		Course:      h.ParseCourseToProto(course, nil),
		Stream:      h.ParseStreamToProto(stream, nil),
		LectureHall: h.ParseLectureHallToProto(lectureHall),
	}, nil
}

// GetVideoSections returns a list of video sections for a stream
func (a *API) GetVideoSections(ctx context.Context, req *protobuf.GetVideoSectionsRequest) (*protobuf.GetVideoSectionsResponse, error) {
	a.log.Info("GetVideoSections")

	_, _, _, err := a.authorizeUserForStreamCourse(ctx, req)
	if err != nil {
		return nil, err
	}

	sections, err := a.dao.GetByStreamId(uint(req.StreamId))
	if err != nil {
		a.log.Error("Can't get video sections", "err", err)
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	var protoSections []*protobuf.VideoSection
	for _, section := range sections {
		protoSections = append(protoSections, h.ParseVideoSectionToProto(section))
	}

	return &protobuf.GetVideoSectionsResponse{Sections: protoSections}, nil
}

// GetStreamPlaylist returns the playlist for a stream
func (a *API) GetStreamPlaylist(ctx context.Context, req *protobuf.GetStreamPlaylistRequest) (*protobuf.GetStreamPlaylistResponse, error) {
	a.log.Info("GetStreamPlaylist")

	user, _, course, err := a.authorizeUserForStreamCourse(ctx, req)
	if err != nil {
		return nil, err
	}

	// Create mapping of stream id to progress for all progresses of user
	var streamIDs []uint
	for _, stream := range course.Streams {
		if stream.Private && (user == nil || !user.IsAdminOfCourse(course)) {
			continue
		}
		streamIDs = append(streamIDs, stream.ID)
	}
	streamProgresses := make(map[uint]model.StreamProgress)
	res, err := a.dao.LoadProgress(user.ID, streamIDs)
	if err != nil {
		a.log.Error("Couldn't load progresses", "err", err)
	} else {
		for _, progress := range res {
			streamProgresses[progress.StreamID] = progress
		}
	}

	var result []*protobuf.StreamPlaylistEntry
	for _, stream := range course.Streams {
		if stream.Private && (user == nil || !user.IsAdminOfCourse(course)) {
			continue
		}
		result = append(result, &protobuf.StreamPlaylistEntry{
			StreamId:       uint32(stream.ID),
			CourseSlug:     course.Slug,
			StreamName:     stream.GetName(),
			LiveNow:        stream.LiveNow,
			Watched:        stream.Watched,
			Start:          timestamppb.New(stream.Start),
			StreamProgress: h.ParseStreamProgressToProto(streamProgresses[stream.ID]),
			CreatedAt:      timestamppb.New(stream.CreatedAt),
		})
	}

	return &protobuf.GetStreamPlaylistResponse{Entries: result}, nil
}

// GetSubtitles returns the subtitles for a stream in a specific language
func (a *API) GetSubtitles(ctx context.Context, req *protobuf.GetSubtitlesRequest) (*httpbody.HttpBody, error) {
	a.log.Info("GetSubtitles")

	_, stream, _, err := a.authorizeUserForStreamCourse(ctx, req)
	if err != nil {
		return nil, err
	}

	lang := req.Lang

	subtitlesObj, err := a.dao.GetByStreamIDandLang(context.Background(), stream.ID, lang)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, e.WithStatus(http.StatusNotFound, err)
		}
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	return &httpbody.HttpBody{
		ContentType: "text/vtt",
		Data:        []byte(subtitlesObj.Content),
	}, nil
}

// GetThumbs returns the thumbnails for a stream
func (a *API) GetThumbs(ctx context.Context, req *protobuf.GetThumbsRequest) (*httpbody.HttpBody, error) {
	a.log.Info("GetThumbs")

	_, stream, _, err := a.authorizeUserForStreamCourse(ctx, req)
	if err != nil {
		return nil, err
	}

	if stream.LiveNow {
		streamId := strconv.Itoa(int(req.StreamId))
		path := pathprovider.LiveThumbnail(streamId)

		data, err := os.ReadFile(path)
		if err != nil {
			return nil, e.WithStatus(http.StatusInternalServerError, err)
		}

		return &httpbody.HttpBody{
			ContentType: "image/jpeg",
			Data:        data,
		}, nil
	}

	if req.ThumbType == nil {
		thumb, err := stream.GetLGThumbnail()
		if err != nil {
			return nil, e.WithStatus(http.StatusNotFound, errors.New("Large Thumbnail not found"))
		}
		data, err := os.ReadFile(thumb)
		if err != nil {
			return nil, e.WithStatus(http.StatusInternalServerError, err)
		}
		return &httpbody.HttpBody{
			ContentType: "image/jpeg",
			Data:        data,
		}, nil
	}

	videoType := model.VideoType(req.GetThumbType().String())
	thumb, err := stream.GetLGThumbnailForVideoType(videoType)
	if err != nil {
		return nil, e.WithStatus(http.StatusNotFound, errors.New("Large Thumbnail not found"))
	}
	data, err := os.ReadFile(thumb)
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	return &httpbody.HttpBody{
		ContentType: "image/jpeg",
		Data:        data,
	}, nil
}
