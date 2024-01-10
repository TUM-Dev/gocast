package actions

import (
	"context"
	"errors"
	"fmt"
	"github.com/tum-dev/gocast/runner/config"
	"log/slog"
	"path"
)

var (
	ErrActionInputWrongType       = errors.New("action input has wrong type")
	ErrRequiredContextValNotFound = errors.New("required context value not found")
)

type ActionProvider struct {
	Log        *slog.Logger
	Cmd        config.CmdList
	SegmentDir string // for storing live hls segments locally. This should be fast storage (e.g. ssd).
	RecDir     string // for storing recordings locally.
	MassDir    string // for storing final files like Thumbnails, mp4s, ... Mass storage like Ceph.
}

func (a *ActionProvider) GetRecDir(courseID, streamID uint64, version string) string {
	return path.Join(a.RecDir, fmt.Sprintf("%d", courseID), fmt.Sprintf("%d", streamID), version)
}

func (a *ActionProvider) GetLiveDir(courseID, streamID uint64, version string) string {
	return path.Join(a.SegmentDir, fmt.Sprintf("%d", courseID), fmt.Sprintf("%d", streamID), version)
}

func (a *ActionProvider) GetMassDir(courseID, streamID uint64, version string) string {
	return path.Join(a.MassDir, fmt.Sprintf("%d", courseID), fmt.Sprintf("%d", streamID), version)
}

type ActionType string

const (
	PrepareAction    ActionType = "prepare"
	StreamAction                = "stream"
	TranscodeAction             = "transcode"
	UploadAction                = "upload"
	ThumbnailAction             = "thumbnail"
	SelfStreamAction            = "selfstream"
)

type Action struct {
	Type     ActionType
	Cancel   context.CancelCauseFunc
	Canceled bool

	ActionFn ActionFn
}

type ActionFn func(ctx context.Context, log *slog.Logger) (context.Context, error)

func set(ctx context.Context, key string, val interface{}) context.Context {
	return context.WithValue(ctx, key, val)
}
