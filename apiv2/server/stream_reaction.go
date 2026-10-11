package apiv2

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"sort"
	"sync"
	"time"

	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/gorm"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/pkg/realtimehub"
	"github.com/TUM-Dev/gocast/tools"
)

const (
	// reactionCooldown is how long a viewer waits between two reactions to the same
	// stream. v1 measured it from the viewer's latest reaction to any stream.
	reactionCooldown = 10 * time.Second

	// reactionWindow is how far back the lecturer's picture of the mood reaches.
	reactionWindow = 15 * time.Minute
)

// reactionStatsInterval is how often administrators are told the split of reactions.
// A variable so the tests of the ticker do not take seconds.
var reactionStatsInterval = 5 * time.Second

// reactionStats sends the course administrators the split of a stream's recent
// reactions, every reactionStatsInterval, while there are any.
//
// v1 ticked for every stream an administrator had registered for, whether anyone had
// reacted or not. This ticks only for streams that were reacted to within the window,
// and its goroutine exists only while there is one: the first reaction starts it, and
// it ends with the last stream's window. The zero value is ready to use.
type reactionStats struct {
	mu      sync.Mutex
	last    map[uint]time.Time // stream -> its latest reaction
	running bool
}

// touch records a reaction to the stream and starts the ticker if none is running.
func (a *API) touchReactionStats(streamID uint) {
	s := &a.reactions
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.last == nil {
		s.last = make(map[uint]time.Time)
	}
	s.last[streamID] = time.Now()
	if s.running {
		return
	}
	s.running = true
	go a.runReactionStats(reactionStatsInterval)
}

func (a *API) runReactionStats(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		if !a.tickReactionStats(context.Background(), time.Now()) {
			return
		}
	}
}

// tickReactionStats publishes the stats of every stream that was reacted to within
// the window, and forgets the others after a last, empty report that lets the
// lecturer's display clear. It reports whether any stream is left to tick for.
func (a *API) tickReactionStats(ctx context.Context, now time.Time) bool {
	s := &a.reactions
	s.mu.Lock()
	var active, expired []uint
	for id, last := range s.last {
		if now.Sub(last) > reactionWindow {
			expired = append(expired, id)
			delete(s.last, id)
		} else {
			active = append(active, id)
		}
	}
	if len(s.last) == 0 {
		// Under the lock, so that a reaction arriving now starts a new ticker.
		s.running = false
	}
	more := len(s.last) > 0
	s.mu.Unlock()

	for _, id := range expired {
		a.publish(ctx, id, realtimehub.ReactionStatsEvent(id, nil, reactionWindow))
	}
	for _, id := range active {
		// Nobody watching, nobody to tell; the stream keeps ticking in case someone
		// joins before its window ends.
		if a.viewers(id) == 0 {
			continue
		}
		reactions, err := a.dao.StreamReactionDao.GetByStreamWithinMinutes(ctx, id, uint(reactionWindow/time.Minute))
		if err != nil {
			a.log.Error("can't load reactions", "err", err, "stream", id)
			continue
		}
		a.publish(ctx, id, realtimehub.ReactionStatsEvent(id, reactionShares(reactions), reactionWindow))
	}
	return more
}

// reactionShares counts reactions per emoji, most used first.
func reactionShares(reactions []model.StreamReaction) []*protobuf.StreamReactionShare {
	counts := make(map[string]uint32)
	for _, r := range reactions {
		counts[r.Reaction]++
	}
	shares := make([]*protobuf.StreamReactionShare, 0, len(counts))
	for reaction, n := range counts {
		shares = append(shares, &protobuf.StreamReactionShare{
			Reaction: reaction, Count: n, Share: float32(n) / float32(len(reactions)),
		})
	}
	sort.Slice(shares, func(i, j int) bool {
		if shares[i].Count != shares[j].Count {
			return shares[i].Count > shares[j].Count
		}
		return shares[i].Reaction < shares[j].Reaction
	})
	return shares
}

// ListAllowedStreamReactions returns the emoji a viewer may react with, replacing
// v1's GET /api/stream/:id/reactions/allowed.
func (a *API) ListAllowedStreamReactions(ctx context.Context, req *protobuf.ListAllowedStreamReactionsRequest) (*protobuf.ListAllowedStreamReactionsResponse, error) {
	if _, _, _, err := a.authorizeUserForStreamCourse(ctx, req); err != nil {
		return nil, err
	}
	return &protobuf.ListAllowedStreamReactionsResponse{Reactions: tools.Cfg.AllowedReactions}, nil
}

// AddStreamReaction records the caller's reaction, replacing v1's addReaction, and
// tells the course's administrators. A viewer has one reaction per stream; reacting
// again replaces it.
func (a *API) AddStreamReaction(ctx context.Context, req *protobuf.AddStreamReactionRequest) (*emptypb.Empty, error) {
	user, stream, _, err := a.authorizeUserForStreamCourse(ctx, req)
	if err != nil {
		return nil, err
	}
	if user == nil {
		// The policy keeps anonymous callers out; this is the second lock.
		return nil, e.WithStatus(http.StatusUnauthorized, errors.New("sign in to react"))
	}
	if !slices.Contains(tools.Cfg.AllowedReactions, req.GetReaction()) {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("reaction not allowed"))
	}

	existing, err := a.dao.StreamReactionDao.GetByStreamAndUser(ctx, stream.ID, user.ID)
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		created := model.StreamReaction{Reaction: req.GetReaction(), StreamID: stream.ID, UserID: user.ID}
		if err := a.dao.StreamReactionDao.Create(ctx, &created); err != nil {
			a.log.Error("can't create reaction", "err", err, "stream", stream.ID)
			return nil, e.WithStatus(http.StatusInternalServerError, errors.New("can't save the reaction"))
		}
	case err != nil:
		a.log.Error("can't look up reaction", "err", err, "stream", stream.ID)
		return nil, e.WithStatus(http.StatusInternalServerError, errors.New("can't save the reaction"))
	default:
		// The cooldown runs from the viewer's last reaction to this stream.
		if existing.UpdatedAt.Add(reactionCooldown).After(time.Now()) {
			return nil, e.WithStatus(http.StatusTooManyRequests, errors.New("please wait a moment before reacting again"))
		}
		existing.Reaction = req.GetReaction()
		// Saved even when the emoji is the same: that refreshes UpdatedAt, which the
		// cooldown and the window of the stats are measured by.
		if err := a.dao.StreamReactionDao.Update(ctx, &existing); err != nil {
			a.log.Error("can't update reaction", "err", err, "stream", stream.ID)
			return nil, e.WithStatus(http.StatusInternalServerError, errors.New("can't save the reaction"))
		}
	}

	a.publish(ctx, stream.ID, realtimehub.ReactionEvent(stream.ID, req.GetReaction()))
	a.touchReactionStats(stream.ID)
	return &emptypb.Empty{}, nil
}
