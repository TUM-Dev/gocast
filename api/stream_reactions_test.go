package api

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/matthiasreumann/gomino"
	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/TUM-Dev/gocast/tools/realtime"
	"github.com/TUM-Dev/gocast/tools/testutils"
)

func StreamReactionRouter(
	t *testing.T,
	reactionDao dao.StreamReactionDao,
) func(r *gin.Engine) {

	return func(r *gin.Engine) {
		wrapper := dao.DaoWrapper{
			StreamsDao:        testutils.GetStreamMock(t),
			CoursesDao:        testutils.GetCoursesMock(t),
			StreamReactionDao: reactionDao,
		}

		configGinStreamRestRouter(r, wrapper)
	}
}

func TestAllowedReactions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tools.Cfg.AllowedReactions = []string{
		"🙂",
		"👍",
		"👎",
		"😳",
	}

	url := fmt.Sprintf(
		"/api/stream/%d/reaction/allowed",
		testutils.StreamFPVLive.ID,
	)

	reactionDao := testutils.GetStreamReactionMock(
		t,
		model.StreamReaction{},
		nil,
	)

	gomino.TestCases{
		"success": {
			Middlewares: testutils.GetMiddlewares(
				tools.ErrorHandler,
				testutils.TUMLiveContext(testutils.TUMLiveContextStudent),
			),

			ExpectedCode: http.StatusOK,
		},
	}.
		Router(StreamReactionRouter(t, reactionDao)).
		Method(http.MethodGet).
		Url(url).
		Run(t, testutils.Equal)
}

func TestAddReaction(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tools.Cfg.AllowedReactions = []string{
		"🙂",
		"👍",
		"👎",
		"😳",
	}

	url := fmt.Sprintf(
		"/api/stream/%d/reaction",
		testutils.StreamFPVLive.ID,
	)

	reactionDao := testutils.GetStreamReactionMock(
		t,
		model.StreamReaction{},
		nil,
	)

	gomino.TestCases{
		"unauthenticated": {
			Middlewares: testutils.GetMiddlewares(
				tools.ErrorHandler,
				testutils.TUMLiveContext(testutils.TUMLiveContextUserNil),
			),
			Body:         `{"reaction":"👍"}`,
			ExpectedCode: http.StatusUnauthorized,
		},

		"invalid_json": {
			Middlewares: testutils.GetMiddlewares(
				tools.ErrorHandler,
				testutils.TUMLiveContext(testutils.TUMLiveContextStudent),
			),
			Body:         "{",
			ExpectedCode: http.StatusBadRequest,
		},

		"invalid_reaction": {
			Middlewares: testutils.GetMiddlewares(
				tools.ErrorHandler,
				testutils.TUMLiveContext(testutils.TUMLiveContextStudent),
			),
			Body:         `{"reaction":"❤️"}`,
			ExpectedCode: http.StatusBadRequest,
		},

		"success": {
			Middlewares: testutils.GetMiddlewares(
				tools.ErrorHandler,
				testutils.TUMLiveContext(testutils.TUMLiveContextStudent),
			),
			Body:         `{"reaction":"👍"}`,
			ExpectedCode: http.StatusOK,
		},
	}.
		Router(StreamReactionRouter(t, reactionDao)).
		Method(http.MethodPost).
		Url(url).
		Run(t, testutils.Equal)
}

func TestAddReactionCooldown(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tools.Cfg.AllowedReactions = []string{
		"🙂",
		"👍",
		"👎",
		"😳",
	}

	url := fmt.Sprintf(
		"/api/stream/%d/reaction",
		testutils.StreamFPVLive.ID,
	)

	reactionDao := testutils.GetStreamReactionMock(
		t,
		model.StreamReaction{
			Model: gorm.Model{
				UpdatedAt: time.Now(),
			},
			Reaction: "👍",
		},
		nil,
	)

	gomino.TestCases{
		"cooldown": {
			Middlewares: testutils.GetMiddlewares(
				tools.ErrorHandler,
				testutils.TUMLiveContext(testutils.TUMLiveContextStudent),
			),
			Body:         `{"reaction":"👍"}`,
			ExpectedCode: http.StatusTooManyRequests,
		},
	}.
		Router(StreamReactionRouter(t, reactionDao)).
		Method(http.MethodPost).
		Url(url).
		Run(t, testutils.Equal)
}

func TestAddReactionCreateFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tools.Cfg.AllowedReactions = []string{
		"🙂",
		"👍",
		"👎",
		"😳",
	}

	url := fmt.Sprintf(
		"/api/stream/%d/reaction",
		testutils.StreamFPVLive.ID,
	)

	reactionDao := testutils.GetStreamReactionMock(
		t,
		model.StreamReaction{},
		errors.New("whoops"),
	)

	gomino.TestCases{
		"create_failure": {
			Middlewares: testutils.GetMiddlewares(
				tools.ErrorHandler,
				testutils.TUMLiveContext(testutils.TUMLiveContextStudent),
			),

			Body:         `{"reaction":"👍"}`,
			ExpectedCode: http.StatusInternalServerError,
		},
	}.
		Router(StreamReactionRouter(t, reactionDao)).
		Method(http.MethodPost).
		Url(url).
		Run(t, testutils.Equal)
}

func TestPruneReactionSessions(t *testing.T) {
	s1 := &realtime.Context{}
	s2 := &realtime.Context{}
	s3 := &realtime.Context{}
	s4 := &realtime.Context{}

	liveReactionListenerMutex.Lock()
	oldListeners := liveReactionListener
	liveReactionListener = map[uint]*liveReactionAdminSessionsWrapper{
		1: {
			sessions: []*realtime.Context{s1, s2},
			stream:   10,
		},
		2: {
			sessions: []*realtime.Context{s3},
			stream:   10,
		},
		3: {
			sessions: []*realtime.Context{s4},
			stream:   11,
		},
	}
	liveReactionListenerMutex.Unlock()

	t.Cleanup(func() {
		liveReactionListenerMutex.Lock()
		liveReactionListener = oldListeners
		liveReactionListenerMutex.Unlock()
	})

	pruneReactionSessions(map[*realtime.Context]struct{}{
		s2: {},
		s3: {},
	})

	liveReactionListenerMutex.RLock()
	defer liveReactionListenerMutex.RUnlock()

	if len(liveReactionListener) != 2 {
		t.Fatalf("expected 2 listeners after prune, got %d", len(liveReactionListener))
	}

	if _, exists := liveReactionListener[2]; exists {
		t.Fatalf("expected user 2 listener to be removed when all sessions are dead")
	}

	listener1, exists := liveReactionListener[1]
	if !exists {
		t.Fatalf("expected user 1 listener to exist")
	}
	if len(listener1.sessions) != 1 || listener1.sessions[0] != s1 {
		t.Fatalf("expected user 1 to keep only the healthy session")
	}

	listener3, exists := liveReactionListener[3]
	if !exists {
		t.Fatalf("expected user 3 listener to exist")
	}
	if len(listener3.sessions) != 1 || listener3.sessions[0] != s4 {
		t.Fatalf("expected user 3 session to remain untouched")
	}
}
