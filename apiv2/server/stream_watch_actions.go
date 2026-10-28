package apiv2

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/meilisearch/meilisearch-go"
	"google.golang.org/protobuf/types/known/emptypb"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/TUM-Dev/gocast/tools/bot"
)

// ---- reporting a problem ----

const (
	// issueCooldown is how long a viewer waits before reporting the same stream again.
	// The reports land in a chat room that operators watch during a lecture.
	issueCooldown = 2 * time.Minute

	maxIssueComment = 2000
	maxIssueContact = 200
)

// issueCategoryLabels is how a category reads in the alert. Keyed by the enum, where
// v1 numbered them in a map that disagreed with the form's own numbering.
var issueCategoryLabels = map[protobuf.StreamIssueCategory]string{
	protobuf.StreamIssueCategory_STREAM_ISSUE_CATEGORY_VIDEO:      "🎬 Video",
	protobuf.StreamIssueCategory_STREAM_ISSUE_CATEGORY_AUDIO:      "🔊 Audio",
	protobuf.StreamIssueCategory_STREAM_ISSUE_CATEGORY_CAMERA:     "🎥 Camera",
	protobuf.StreamIssueCategory_STREAM_ISSUE_CATEGORY_MICROPHONE: "🎤 Microphone",
	protobuf.StreamIssueCategory_STREAM_ISSUE_CATEGORY_LIGHT:      "💡 Light",
	protobuf.StreamIssueCategory_STREAM_ISSUE_CATEGORY_OTHER:      "Other",
}

// issueLimiter remembers when a viewer last reported a stream. The zero value is ready.
type issueLimiter struct {
	mu   sync.Mutex
	last map[[2]uint]time.Time // {user, stream}
}

// allow records a report and reports whether it may go ahead. Expired entries are
// dropped as they are met, which bounds the map by the reports of one cooldown.
func (l *issueLimiter) allow(userID, streamID uint, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.last == nil {
		l.last = make(map[[2]uint]time.Time)
	}
	for k, t := range l.last {
		if now.Sub(t) >= issueCooldown {
			delete(l.last, k)
		}
	}
	key := [2]uint{userID, streamID}
	if _, cooling := l.last[key]; cooling {
		return false
	}
	l.last[key] = now
	return true
}

// release gives the slot back, for a report that did not go out.
func (l *issueLimiter) release(userID, streamID uint) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.last, [2]uint{userID, streamID})
}

// alertMu serialises bot.SendAlert, which keeps its report history in an unguarded
// package map that v1's single-threaded use never contended.
var alertMu sync.Mutex

// sendMatrixAlert is how a report reaches the operators.
func (a *API) sendMatrixAlert(msg bot.AlertMessage) error {
	alertMu.Lock()
	defer alertMu.Unlock()

	var b bot.Bot
	b.SetMessagingMethod(&bot.Matrix{})
	return b.SendAlert(msg, a.dao.StatisticsDao)
}

// ReportStreamIssue forwards a viewer's problem report to the alert room, replacing
// v1's reportStreamIssue. Open to every signed-in viewer who may watch the stream:
// v1 allowed only the course's administrators, so a student's report was a 403.
func (a *API) ReportStreamIssue(ctx context.Context, req *protobuf.ReportStreamIssueRequest) (*emptypb.Empty, error) {
	user, stream, course, err := a.authorizeUserForStreamCourse(ctx, req)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, e.WithStatus(http.StatusUnauthorized, errors.New("sign in to report a problem"))
	}
	if stream.LectureHallID == 0 {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("only streams held in a lecture hall can be reported"))
	}
	comment := strings.TrimSpace(req.GetComment())
	if comment == "" {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("describe the problem"))
	}
	if utf8.RuneCountInString(comment) > maxIssueComment {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("the description is too long"))
	}
	for _, contact := range []string{req.GetName(), req.GetEmail(), req.GetPhone()} {
		if utf8.RuneCountInString(contact) > maxIssueContact {
			return nil, e.WithStatus(http.StatusBadRequest, errors.New("a contact detail is too long"))
		}
	}
	labels := make([]string, 0, len(req.GetCategories()))
	for _, c := range req.GetCategories() {
		label, ok := issueCategoryLabels[c]
		if !ok {
			return nil, e.WithStatus(http.StatusBadRequest, errors.New("unknown category"))
		}
		labels = append(labels, label)
	}

	// After the request's own faults: whether the deployment can take a report is not
	// something a client can correct.
	if !alertsConfigured() {
		return nil, e.WithStatus(http.StatusServiceUnavailable, errors.New("problem reports are not set up"))
	}

	hall, err := a.dao.LectureHallsDao.GetLectureHallByID(stream.LectureHallID)
	if err != nil {
		return nil, e.FromGorm(err, "can't find the stream's lecture hall")
	}

	if !a.issues.allow(user.ID, stream.ID, time.Now()) {
		return nil, e.WithStatus(http.StatusTooManyRequests, errors.New("you reported this stream a moment ago, please wait"))
	}

	send := a.sendAlert
	if send == nil {
		send = a.sendMatrixAlert
	}
	// e.g. https://live.rbg.tum.de/w/gbs/1234
	if err := send(bot.AlertMessage{
		PhoneNumber: req.GetPhone(),
		Name:        req.GetName(),
		Email:       req.GetEmail(),
		Comment:     comment,
		Categories:  strings.Join(labels, " · "),
		CourseName:  course.Name,
		LectureHall: hall.Name,
		StreamUrl:   fmt.Sprintf("%s/w/%s/%d", tools.Cfg.WebUrl, course.Slug, stream.ID),
		CombIP:      hall.CombIP,
		CameraIP:    hall.CameraIP,
		IsLecturer:  user.CanAdminister(course),
		Stream:      stream,
		User:        *user,
	}); err != nil {
		a.issues.release(user.ID, stream.ID)
		a.log.Error("can't send the alert", "err", err, "stream", stream.ID)
		return nil, e.WithStatus(http.StatusInternalServerError, errors.New("can't send the report"))
	}
	return &emptypb.Empty{}, nil
}

// ---- seeks ----

const (
	// A heatmap of too few seeks says nothing. Both must be reached, as in v1.
	seekHeatmapMinHits   = 150
	seekHeatmapMinChunks = 50
)

// ReportStreamSeek counts one seek for the heatmap, replacing v1's POST
// /api/seekReport/:id. v1 neither checked the viewer could see the stream nor
// that the position was a number within it.
func (a *API) ReportStreamSeek(ctx context.Context, req *protobuf.ReportStreamSeekRequest) (*emptypb.Empty, error) {
	_, stream, _, err := a.authorizeUserForStreamCourse(ctx, req)
	if err != nil {
		return nil, err
	}

	pos := req.GetPosition()
	// Without a duration there are no chunks to count in (the recording has not been
	// processed, or it is live).
	if math.IsNaN(pos) || pos < 0 || !stream.Duration.Valid || stream.Duration.Int32 <= 0 || pos > float64(stream.Duration.Int32) {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("position is not within the recording"))
	}

	if err := a.dao.VideoSeekDao.Add(streamIDString(req.GetStreamId()), pos); err != nil {
		if errors.Is(err, dao.ErrPositionOutOfRange) {
			return nil, e.WithStatus(http.StatusBadRequest, err)
		}
		a.log.Error("can't count seek", "err", err, "stream", stream.ID)
		return nil, e.WithStatus(http.StatusInternalServerError, errors.New("can't count the seek"))
	}
	return &emptypb.Empty{}, nil
}

// GetStreamSeekHeatmap returns where viewers jump to, replacing v1's GET
// /api/seekReport/:id.
func (a *API) GetStreamSeekHeatmap(ctx context.Context, req *protobuf.GetStreamSeekHeatmapRequest) (*protobuf.GetStreamSeekHeatmapResponse, error) {
	if _, _, _, err := a.authorizeUserForStreamCourse(ctx, req); err != nil {
		return nil, err
	}

	chunks, err := a.dao.VideoSeekDao.Get(streamIDString(req.GetStreamId()))
	if err != nil {
		a.log.Error("can't load seeks", "err", err, "stream", req.GetStreamId())
		return nil, e.WithStatus(http.StatusInternalServerError, errors.New("can't load the seeks"))
	}

	res := &protobuf.GetStreamSeekHeatmapResponse{BucketCount: dao.SeekHeatmapChunks}
	var buckets []*protobuf.SeekHeatmapBucket
	hits, nonZero := 0, 0
	for _, c := range chunks {
		if c.Hits == 0 || c.ChunkIndex >= dao.SeekHeatmapChunks {
			continue
		}
		hits += int(c.Hits)
		nonZero++
		buckets = append(buckets, &protobuf.SeekHeatmapBucket{Index: uint32(c.ChunkIndex), Hits: uint32(c.Hits)})
	}
	if hits >= seekHeatmapMinHits && nonZero >= seekHeatmapMinChunks {
		res.Buckets = buckets
	}
	return res, nil
}

// ---- subtitles ----

const (
	subtitleSearchMinQuery     = 3
	subtitleSearchDefaultLimit = 10
	subtitleSearchMaxLimit     = 100
)

// WithSubtitleSearch gives the API its Meilisearch for searching inside one
// recording's subtitles. Without one, that search answers 503.
func WithSubtitleSearch(m tools.MeiliSearchInterface) Option {
	return func(a *API) { a.subtitleSearch = m }
}

// SearchStreamSubtitles finds lines of one recording's subtitles, replacing v1's
// /api/search/stream/:id/subtitles. v1 returned Meilisearch's response as it came,
// ten hits at most.
func (a *API) SearchStreamSubtitles(ctx context.Context, req *protobuf.SearchStreamSubtitlesRequest) (*protobuf.SearchStreamSubtitlesResponse, error) {
	_, stream, _, err := a.authorizeUserForStreamCourse(ctx, req)
	if err != nil {
		return nil, err
	}

	query := strings.TrimSpace(req.GetQuery())
	if utf8.RuneCountInString(query) < subtitleSearchMinQuery {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("the query needs three characters or more"))
	}
	limit := int64(req.GetLimit())
	if limit <= 0 {
		limit = subtitleSearchDefaultLimit
	}
	limit = min(limit, subtitleSearchMaxLimit)

	if a.subtitleSearch == nil {
		return nil, e.WithStatus(http.StatusServiceUnavailable, errors.New("search is not available"))
	}
	// The stream is the filter, so no other recording's lines can come back.
	res := a.subtitleSearch.Search(query, limit, 1, "", "", fmt.Sprintf("streamID = %d", stream.ID))
	if res == nil {
		return nil, e.WithStatus(http.StatusServiceUnavailable, errors.New("search is not available"))
	}

	out := &protobuf.SearchStreamSubtitlesResponse{}
	for _, r := range res.Results {
		if r.IndexUID != "SUBTITLES" {
			continue
		}
		hits, err := subtitleHits(r.Hits)
		if err != nil {
			a.log.Error("can't read subtitle hits", "err", err, "stream", stream.ID)
			return nil, e.WithStatus(http.StatusInternalServerError, errors.New("can't read the search results"))
		}
		out.Hits = append(out.Hits, hits...)
	}
	return out, nil
}

func subtitleHits(hits []meilisearch.Hit) ([]*protobuf.StreamSubtitleHit, error) {
	if len(hits) == 0 {
		return nil, nil
	}
	raw, err := json.Marshal(hits)
	if err != nil {
		return nil, err
	}
	var parsed []struct {
		Timestamp int64  `json:"timestamp"`
		TextPrev  string `json:"textPrev"`
		Text      string `json:"text"`
		TextNext  string `json:"textNext"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}
	out := make([]*protobuf.StreamSubtitleHit, 0, len(parsed))
	for _, h := range parsed {
		out = append(out, &protobuf.StreamSubtitleHit{
			Timestamp: h.Timestamp, TextPrev: h.TextPrev, Text: h.Text, TextNext: h.TextNext,
		})
	}
	return out, nil
}

// ---- ending a stream ----

// StreamEnder stops a live stream where it is being recorded.
// *runner_manager.Manager satisfies it.
type StreamEnder interface {
	EndStream(ctx context.Context, streamID uint, discardVoD bool) error
}

// WithStreamEnder gives the API the means to end a live stream. Without one,
// endStream answers 503.
func WithStreamEnder(s StreamEnder) Option {
	return func(a *API) { a.streamEnder = s }
}

// EndStream ends a live stream early, replacing v1's GET /api/stream/:id/end. The
// manager marks the stream not live and ended, and tells the viewers over the realtime
// hub through its live-state notifier (see runner_manager.WithLiveStateNotifier), so
// this publishes nothing itself: a second StreamLiveEvent would only repeat it.
//
// v1 took the stream from the URL without comparing it to the course, and ended it
// whether it was live or not; ending a stream that is not live would still mark it
// ended and stop a recording that has not started.
func (a *API) EndStream(ctx context.Context, req *protobuf.EndStreamRequest) (*emptypb.Empty, error) {
	stream, err := a.courseLecture(ctx, req.GetCourseId(), req.GetStreamId())
	if err != nil {
		return nil, err
	}
	if !stream.LiveNow {
		return nil, e.WithStatus(http.StatusBadRequest, errStreamNotLive)
	}
	if a.streamEnder == nil {
		return nil, e.WithStatus(http.StatusServiceUnavailable, errors.New("streams cannot be ended here"))
	}

	if err := a.streamEnder.EndStream(ctx, stream.ID, req.GetDiscard()); err != nil {
		a.log.Error("can't end the stream", "err", err, "stream", stream.ID)
		return nil, e.WithStatus(http.StatusInternalServerError, errors.New("can't end the stream"))
	}
	a.audit(ctx, model.AuditStreamEdit, fmt.Sprintf("%d: ended (discard: %v)", stream.ID, req.GetDiscard()), forStream(stream))
	return &emptypb.Empty{}, nil
}

// streamIDString is the form the streams and seek DAOs take ids in.
func streamIDString(id uint32) string { return fmt.Sprintf("%d", id) }
