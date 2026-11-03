package runner_manager

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/tum-dev/gocast/runner/protobuf"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/model"
)

// spriteFileType is how the sprite of a version is stored, see Stream.GetThumbIdForSource.
func spriteFileType(version protobuf.StreamVersion) (model.FileType, bool) {
	switch version {
	case protobuf.StreamVersion_STREAM_VERSION_COMBINED:
		return model.FILETYPE_THUMB_COMB, true
	case protobuf.StreamVersion_STREAM_VERSION_CAMERA:
		return model.FILETYPE_THUMB_CAM, true
	case protobuf.StreamVersion_STREAM_VERSION_PRESENTATION:
		return model.FILETYPE_THUMB_PRES, true
	}
	return 0, false
}

// saveSprite stores the sprite of one version and the interval of its frames. The runner
// repeats a notification it is unsure was delivered, and replacing makes that harmless.
func (m *Manager) saveSprite(ctx context.Context, req *protobuf.SpriteReadyNotification) error {
	fileType, ok := spriteFileType(req.GetStreamVersion())
	if !ok {
		return status.Errorf(codes.InvalidArgument, "invalid stream version %v", req.GetStreamVersion())
	}
	if len(req.GetSprite()) == 0 || req.GetIntervalSeconds() == 0 {
		return status.Errorf(codes.InvalidArgument, "sprite of %d bytes, a frame every %d seconds", len(req.GetSprite()), req.GetIntervalSeconds())
	}
	stream, err := m.dao.StreamsDao.GetStreamByID(ctx, strconv.FormatUint(req.GetStream().GetId(), 10))
	if err != nil {
		return lookupError(err, "stream %d", req.GetStream().GetId())
	}

	// One interval serves all versions of a stream. Replacing it would make another version's
	// older sprite show the wrong frames, so this version goes without a preview instead.
	if stream.ThumbInterval != 0 && stream.ThumbInterval != req.GetIntervalSeconds() && hasOtherSprite(stream, fileType) {
		m.logger.Warn("dropping a sprite whose interval differs from the stream's", "stream", stream.ID,
			"version", req.GetStreamVersion().String(), "interval", req.GetIntervalSeconds(), "stream_interval", stream.ThumbInterval)
		return nil
	}

	dir := filepath.Join(m.massStorage, "thumbs", stream.Start.Format("2006/01"), fmt.Sprintf("%d", stream.CourseID))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return status.Errorf(codes.Internal, "can't make directory: %v", err)
	}
	name := fmt.Sprintf("%d_%s_sprite.jpeg", stream.ID, req.GetStreamVersion().String())
	path := filepath.Join(dir, name)
	err = writeFileAtomic(path, func(w io.Writer) error {
		_, err := w.Write(req.GetSprite())
		return err
	})
	if err != nil {
		return status.Errorf(codes.Internal, "can't write file: %v", err)
	}

	// The interval first: a sprite is only offered with one, so a failure between the two
	// leaves no file with the wrong interval, and the retry repeats both.
	if err := m.dao.StreamsDao.SetThumbInterval(stream.ID, req.GetIntervalSeconds()); err != nil {
		return status.Errorf(codes.Internal, "can't save the sprite's interval: %v", err)
	}
	err = m.dao.FileDao.SetThumbnail(stream.ID, model.File{StreamID: stream.ID, Path: path, Filename: name, Type: fileType})
	if err != nil {
		return status.Errorf(codes.Internal, "can't save sprite to db: %v", err)
	}
	return nil
}

// hasOtherSprite tells whether a version other than the one of fileType has a sprite.
func hasOtherSprite(stream model.Stream, fileType model.FileType) bool {
	for _, f := range stream.Files {
		switch f.Type {
		case model.FILETYPE_THUMB_COMB, model.FILETYPE_THUMB_CAM, model.FILETYPE_THUMB_PRES:
			if f.Type != fileType {
				return true
			}
		}
	}
	return false
}

// needsSprite tells whether redoing the post-processing of a version should make a sprite.
// Only one that has none: most older recordings have a good one that the worker made, at an
// interval of its own that all versions of the stream share, and making a sheet reads the
// whole video. A failed lookup counts as having one, since the next redo can still make it.
func (m *Manager) needsSprite(streamID uint, version model.StreamVersion) bool {
	fileType, ok := spriteFileType(protobufStreamVersion(version))
	if !ok {
		return false
	}
	_, err := m.dao.FileDao.GetThumbnail(streamID, fileType)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		m.logger.Error("can't tell whether a stream has a sprite", "stream", streamID, "version", version, "err", err)
	}
	return errors.Is(err, gorm.ErrRecordNotFound)
}
