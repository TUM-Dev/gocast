package apiv2

import (
	"context"
	"math"
	"strings"

	"google.golang.org/protobuf/types/known/timestamppb"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
)

// presetPlaceholder is the image of a camera preset nobody has photographed yet, as
// the old page showed it. Served from the static directory, like the snapshots.
const presetPlaceholder = "noPreset.jpg"

// GetStreamWatch returns everything the watch page renders in one call, replacing
// web/watch.go's WatchPage and the template logic around it. Which layout to show is
// the URL's business and the viewer's saved preference, so v1's redirect to the
// preferred view is not here: default_layout says what to open with, and the client
// decides.
func (a *API) GetStreamWatch(ctx context.Context, req *protobuf.GetStreamWatchRequest) (*protobuf.StreamWatch, error) {
	user, stream, course, err := a.authorizeUserForStreamCourse(ctx, req)
	if err != nil {
		return nil, err
	}

	signedIn := user != nil
	isAdmin := user.CanAdminister(course)

	var hall *model.LectureHall
	if stream.LectureHallID != 0 {
		lh, err := a.dao.LectureHallsDao.GetLectureHallByID(stream.LectureHallID)
		if err != nil {
			// The page is still worth showing without the hall's name and presets.
			a.log.Error("can't load the stream's lecture hall", "err", err, "stream", stream.ID)
		} else {
			hall = &lh
		}
	}

	// Recordings may be downloaded when the course allows it to signed-in viewers, or
	// to its administrators. The permission is baked into the signed playlist, which
	// is what the edge checks, so it is decided before signing. v1 let anonymous
	// callers have it too whenever the course allowed downloads.
	canDownload := stream.IsDownloadable() && ((course.DownloadsEnabled && signedIn) || isAdmin)
	if err := tools.SetSignedPlaylistsForCourse(&stream, user, course.Slug, canDownload); err != nil {
		// Unsigned playlists are refused by the edge, so the page will show an error
		// where the player should be, which is what v1 did too.
		a.log.Warn("can't sign playlists", "err", err, "stream", stream.ID)
	}

	sources := watchSources(&stream, &course)
	watch := &protobuf.StreamWatch{
		Stream:  watchStream(&stream, hall),
		Course:  watchCourse(&course),
		Viewers: uint32(a.viewers(stream.ID)),
		Chat: &protobuf.WatchChat{
			// Either switch turns the chat on, as sendChatMessage decides.
			Enabled:          course.ChatEnabled || stream.ChatEnabled,
			AnonymousAllowed: course.AnonymousChatEnabled,
			ApprovalRequired: course.ModeratedChatEnabled,
		},
		Viewer: &protobuf.WatchViewer{
			SignedIn:       signedIn,
			IsCourseAdmin:  isAdmin,
			CanReportIssue: alertsConfigured() && signedIn && stream.LectureHallID != 0,
		},
	}

	if stream.LiveNow || stream.Recording {
		watch.Views = watchViews(&stream, sources)
	}
	watch.SplitAvailable = hasKind(watch.Views, protobuf.WatchViewKind_WATCH_VIEW_KIND_PRESENTATION) &&
		hasKind(watch.Views, protobuf.WatchViewKind_WATCH_VIEW_KIND_CAMERA)
	watch.DefaultLayout = defaultLayout(watch.Views, watch.SplitAvailable, user)

	if stream.Recording && !stream.LiveNow {
		watch.Silences = watchSilences(&stream)
		// Offsets are milliseconds and absolute positions: the cut editor stored the
		// end as the position, not as a duration.
		if s := uint32(stream.StartOffset / 1000); s > 0 {
			watch.StartOffset = &s
		}
		if end := uint32(stream.EndOffset / 1000); end > 0 {
			watch.EndOffset = &end
		}
		if canDownload {
			watch.Downloads = watchDownloads(sources)
		}
		if langs, err := a.dao.SubtitlesDao.GetLanguagesByStreamID(ctx, stream.ID); err != nil {
			a.log.Error("can't list subtitle languages", "err", err, "stream", stream.ID)
		} else {
			watch.SubtitleLanguages = langs
		}
	}

	for _, f := range stream.Attachments() {
		att := &protobuf.WatchAttachment{FileId: uint32(f.ID), Filename: f.Filename}
		if att.Filename == "" {
			att.Filename = f.GetDownloadFileName()
		}
		// model.File.IsURL compares the scheme with "https://" and so never matches;
		// v1 served links through the download endpoint as a result. Not fixed there
		// (v1 is going away), read correctly here.
		if strings.HasPrefix(f.Path, "https://") || strings.HasPrefix(f.Path, "http://") {
			link := f.Path
			att.ExternalUrl = &link
		}
		watch.Attachments = append(watch.Attachments, att)
	}

	if signedIn && stream.Recording {
		progress := model.StreamProgress{StreamID: stream.ID}
		if found, err := a.dao.LoadProgress(user.ID, []uint{stream.ID}); err != nil {
			a.log.Warn("can't load progress", "err", err, "stream", stream.ID)
		} else if len(found) > 0 {
			progress = found[0]
		}
		watch.Progress = &protobuf.StreamProgress{
			StreamId: uint32(stream.ID),
			Progress: float32(progress.Progress),
			Watched:  progress.Watched,
		}

		// The length of a self-stream's slot says nothing about its video, so only a
		// recording from a lecture hall has a first silence worth skipping.
		if stream.LectureHallID != 0 {
			if p := stream.FirstSilenceAsProgress(); p > 0 && !math.IsNaN(p) {
				p32 := float32(math.Min(p, 1))
				watch.FirstSilenceEndProgress = &p32
			}
		}
	}

	if isAdmin {
		watch.Admin = &protobuf.WatchAdmin{CanEndStream: stream.LiveNow && stream.LectureHallID != 0}
		if stream.LectureHallID != 0 {
			id := uint32(stream.LectureHallID)
			watch.Admin.LectureHallId = &id
		}
		// Moving the camera only makes sense while the room is streaming.
		if stream.LiveNow && hall != nil {
			for _, p := range hall.CameraPresets {
				image := p.Image
				if image == "" {
					image = presetPlaceholder
				}
				watch.Admin.CameraPresets = append(watch.Admin.CameraPresets, &protobuf.WatchCameraPreset{
					PresetId:  uint32(p.PresetID),
					Name:      p.Name,
					ImageUrl:  "/public/" + image,
					IsDefault: p.IsDefault,
				})
			}
		}
	}

	return watch, nil
}

// viewers is how many sockets watch the stream; zero for an API without a hub.
func (a *API) viewers(streamID uint) int {
	if a.hub == nil {
		return 0
	}
	return a.hub.Viewers(streamID)
}

// alertsConfigured reports whether the deployment has somewhere to send problem
// reports to. A variable so that tests need not build the config's anonymous structs.
var alertsConfigured = func() bool {
	return tools.Cfg.Alerts != nil && tools.Cfg.Alerts.Matrix != nil
}

// watchSource is one playable recording or live feed of a stream.
type watchSource struct {
	kind     protobuf.WatchViewKind
	name     string // as the download menu shows it
	playlist string
	thumb    string // the source's name for model.Stream.GetThumbIdForSource
}

// watchSources lists the stream's playlists that the course lets the viewer have,
// combined first. A lecture hall the course sets to presentation only (source mode 1)
// or camera only (2) yields just that source, whatever else was recorded; a stream in
// no hall has no such setting. Call after signing: the URLs are the signed ones.
func watchSources(s *model.Stream, c *model.Course) []watchSource {
	mode := model.SourceModeCOMB
	if s.LectureHallID != 0 {
		mode = c.GetSourceModeForLectureHall(s.LectureHallID)
	}

	var out []watchSource
	add := func(src watchSource) {
		if src.playlist != "" {
			out = append(out, src)
		}
	}
	if mode == model.SourceModeCOMB {
		add(watchSource{protobuf.WatchViewKind_WATCH_VIEW_KIND_COMBINED, "Combined", s.PlaylistUrl, "COMB"})
	}
	if mode != model.SourceModeCAMOnly {
		add(watchSource{protobuf.WatchViewKind_WATCH_VIEW_KIND_PRESENTATION, "Presentation", s.PlaylistUrlPRES, "PRES"})
	}
	if mode != model.SourceModePRESOnly {
		add(watchSource{protobuf.WatchViewKind_WATCH_VIEW_KIND_CAMERA, "Camera", s.PlaylistUrlCAM, "CAM"})
	}
	return out
}

func watchViews(s *model.Stream, sources []watchSource) []*protobuf.WatchView {
	views := make([]*protobuf.WatchView, 0, len(sources))
	for _, src := range sources {
		view := &protobuf.WatchView{Kind: src.kind, PlaylistUrl: src.playlist}
		// A live stream has no sprite yet, and a recording whose sprite is still
		// being made has no file for it.
		if !s.LiveNow && s.ThumbInterval > 0 {
			if id := s.GetThumbIdForSource(src.thumb); id != model.FILETYPE_INVALID {
				view.Sprite = &protobuf.WatchSprite{FileId: uint32(id), IntervalSeconds: s.ThumbInterval}
			}
		}
		views = append(views, view)
	}
	return views
}

func watchDownloads(sources []watchSource) []*protobuf.WatchDownload {
	var out []*protobuf.WatchDownload
	for _, src := range sources {
		out = append(out, &protobuf.WatchDownload{
			Kind:         src.kind,
			FriendlyName: src.name,
			Url:          src.playlist + "&download=1",
		})
	}
	return out
}

func hasKind(views []*protobuf.WatchView, kind protobuf.WatchViewKind) bool {
	for _, v := range views {
		if v.Kind == kind {
			return true
		}
	}
	return false
}

// defaultLayout picks what to open with when the URL names no layout: the viewer's
// saved LECTURE_VIEW, narrowed to what the views can show. A view that is not there
// (the course allows only the presentation) falls back to what is.
func defaultLayout(views []*protobuf.WatchView, splitAvailable bool, user *model.User) protobuf.WatchLayout {
	if len(views) == 0 {
		return protobuf.WatchLayout_WATCH_LAYOUT_UNSPECIFIED
	}

	wanted := protobuf.WatchLayout_WATCH_LAYOUT_COMBINED
	if user != nil {
		switch user.GetPreferredView() {
		case "Presentation":
			wanted = protobuf.WatchLayout_WATCH_LAYOUT_PRESENTATION
		case "Camera":
			wanted = protobuf.WatchLayout_WATCH_LAYOUT_CAMERA
		case "Split":
			wanted = protobuf.WatchLayout_WATCH_LAYOUT_SPLIT
		}
	}

	showable := func(l protobuf.WatchLayout) bool {
		switch l {
		case protobuf.WatchLayout_WATCH_LAYOUT_COMBINED:
			return hasKind(views, protobuf.WatchViewKind_WATCH_VIEW_KIND_COMBINED)
		case protobuf.WatchLayout_WATCH_LAYOUT_PRESENTATION:
			return hasKind(views, protobuf.WatchViewKind_WATCH_VIEW_KIND_PRESENTATION)
		case protobuf.WatchLayout_WATCH_LAYOUT_CAMERA:
			return hasKind(views, protobuf.WatchViewKind_WATCH_VIEW_KIND_CAMERA)
		case protobuf.WatchLayout_WATCH_LAYOUT_SPLIT:
			return splitAvailable
		}
		return false
	}

	if showable(wanted) {
		return wanted
	}
	for _, l := range []protobuf.WatchLayout{
		protobuf.WatchLayout_WATCH_LAYOUT_COMBINED,
		protobuf.WatchLayout_WATCH_LAYOUT_PRESENTATION,
		protobuf.WatchLayout_WATCH_LAYOUT_CAMERA,
	} {
		if showable(l) {
			return l
		}
	}
	return protobuf.WatchLayout_WATCH_LAYOUT_UNSPECIFIED
}

func watchStream(s *model.Stream, hall *model.LectureHall) *protobuf.WatchStream {
	out := &protobuf.WatchStream{
		Id:              uint32(s.ID),
		Name:            s.Name,
		DisplayName:     s.GetName(),
		DescriptionHtml: s.GetDescriptionHTML(),
		Start:           timestamppb.New(s.Start),
		End:             timestamppb.New(s.End),
		LiveNow:         s.LiveNow,
		Recording:       s.Recording,
		Ended:           s.IsPast(),
		IsComingUp:      s.IsComingUp(),
		IsPlanned:       s.IsPlanned(),
		IsPrivate:       s.Private,
	}
	if hall != nil {
		name := hall.Name
		out.LectureHallName = &name
	}
	if s.Duration.Valid && s.Duration.Int32 > 0 {
		d := uint32(s.Duration.Int32)
		out.Duration = &d
	}
	return out
}

func watchCourse(c *model.Course) *protobuf.WatchCourse {
	return &protobuf.WatchCourse{
		Id:   uint32(c.ID),
		Name: c.Name,
		Slug: c.Slug,
		Year: uint32(c.Year),
		Term: c.TeachingTerm,
	}
}

func watchSilences(s *model.Stream) []*protobuf.WatchSilence {
	out := make([]*protobuf.WatchSilence, 0, len(s.Silences))
	for _, sil := range s.Silences {
		out = append(out, &protobuf.WatchSilence{Start: uint32(sil.Start), End: uint32(sil.End)})
	}
	return out
}
