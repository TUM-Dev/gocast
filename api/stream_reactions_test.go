package api

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/TUM-Dev/gocast/tools/testutils"
	"github.com/gin-gonic/gin"
	"github.com/matthiasreumann/gomino"
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
				CreatedAt: time.Now(),
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
