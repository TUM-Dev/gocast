package runner_manager

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"

	"github.com/tum-dev/gocast/runner/pkg/ptr"
	"github.com/tum-dev/gocast/runner/protobuf"

	"github.com/TUM-Dev/gocast/model"
)

// SectionImageRequest describes the video sections of a stream that need a thumbnail.
type SectionImageRequest struct {
	StreamID    uint
	PlaylistURL string
	Sections    []model.VideoSection
}

// RequestSectionImages asks a runner to generate thumbnails for the given video sections.
// The runner answers asynchronously with a SectionImagesReadyNotification, which
// saveSectionImages then stores, so this returns as soon as the job is accepted.
func (m *Manager) RequestSectionImages(ctx context.Context, req SectionImageRequest) error {
	if len(req.Sections) == 0 {
		return nil
	}

	runner, client, conn, err := m.getClient(ctx)
	if err != nil {
		return fmt.Errorf("getClient: %w", err)
	}
	defer func() {
		_ = conn.Close()
	}()

	sections := make([]*protobuf.SectionTimestamp, 0, len(req.Sections))
	for _, s := range req.Sections {
		sections = append(sections, &protobuf.SectionTimestamp{
			SectionId: ptr.Take(uint64(s.ID)),
			Hours:     ptr.Take(uint32(s.StartHours)),
			Minutes:   ptr.Take(uint32(s.StartMinutes)),
			Seconds:   ptr.Take(uint32(s.StartSeconds)),
		})
	}

	ctx, cancel := context.WithTimeout(ctx, runnerDispatchTimeout)
	defer cancel()

	resp, err := client.RequestSectionImages(ctx, &protobuf.SectionImageRequest{
		StreamId:    ptr.Take(uint64(req.StreamID)),
		PlaylistUrl: ptr.Take(req.PlaylistURL),
		Sections:    sections,
	})
	if err != nil {
		return fmt.Errorf("RequestSectionImages: %w", err)
	}
	m.logger.With("stream", req.StreamID, "job", resp.GetJobId(), "runner", runner.Hostname).
		Info("requested section images")
	return nil
}

// saveSectionImages writes the thumbnails a runner generated to mass storage and points
// the video sections at them.
//
// The runner sends the images as bytes and gocast picks the location, so the path is
// built from ids only and never from free text such as the course name.
func (m *Manager) saveSectionImages(ctx context.Context, req *protobuf.SectionImagesReadyNotification) error {
	stream, err := m.dao.StreamsDao.GetStreamByID(ctx, strconv.FormatUint(req.GetStream().GetId(), 10))
	if err != nil {
		return status.Errorf(codes.NotFound, "can't find stream for id %d: %v", req.GetStream().GetId(), err)
	}

	dir := filepath.Join(m.massStorage, "sections", stream.Start.Format("2006/01"), strconv.FormatUint(uint64(stream.CourseID), 10))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return status.Errorf(codes.Internal, "can't make directory: %v", err)
	}

	for _, image := range req.GetImages() {
		sectionID := uint(image.GetSectionId())
		// /mass/sections/2025/10/500/1024_42.jpg
		fname := fmt.Sprintf("%d_%d.jpg", stream.ID, sectionID)
		path := filepath.Join(dir, fname)

		// Sections are regenerated whenever the thumbnails are redone. The path only
		// depends on ids, so the image on disk is replaced in place and just the file
		// row it used to point at has to go.
		var previousFileID uint
		if section, err := m.dao.VideoSectionDao.Get(sectionID); err == nil {
			previousFileID = section.FileID
		}

		if err := os.WriteFile(path, image.GetImage(), 0o644); err != nil {
			return status.Errorf(codes.Internal, "can't write section image: %v", err)
		}

		file := model.File{
			StreamID: stream.ID,
			Path:     path,
			Filename: fname,
			Type:     model.FILETYPE_IMAGE_JPG,
		}
		if err := m.dao.FileDao.NewFile(&file); err != nil {
			return status.Errorf(codes.Internal, "can't save section image to db: %v", err)
		}

		update := model.VideoSection{Model: gorm.Model{ID: sectionID}, FileID: file.ID}
		if err := m.dao.VideoSectionDao.Update(&update); err != nil {
			return status.Errorf(codes.Internal, "can't update video section %d: %v", sectionID, err)
		}

		// Only now that nothing points at it any more. Losing this is not worth failing
		// the notification over, the image itself is already in place.
		if previousFileID != 0 && previousFileID != file.ID {
			if err := m.dao.FileDao.DeleteFile(previousFileID); err != nil {
				m.logger.Warn("can't delete replaced section image file", "file", previousFileID, "err", err)
			}
		}
	}

	m.logger.With("stream", stream.ID, "images", len(req.GetImages())).Info("saved section images")
	return nil
}
