package api

import (
	"errors"
	"github.com/getsentry/sentry-go"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	"gorm.io/gorm"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

var progressBuff *progressBuffer

// progressBuffer contains progresses to be written to the database
type progressBuffer struct {
	lock       sync.Mutex
	progresses []model.StreamProgress
	interval   time.Duration
}

func newProgressBuffer() *progressBuffer {
	return &progressBuffer{
		lock:       sync.Mutex{},
		progresses: []model.StreamProgress{},
		interval:   time.Second * 5,
	}
}

// add new progress to the list to be flushed eventually
func (b *progressBuffer) add(progress model.StreamProgress) {
	b.lock.Lock()
	defer b.lock.Unlock()
	b.progresses = append(b.progresses, progress)
}

// flush writes the collected progresses to the database
func (b *progressBuffer) flush() error {
	b.lock.Lock()
	defer b.lock.Unlock()
	if len(b.progresses) == 0 {
		return nil
	}
	err := dao.Progress.SaveProgresses(b.progresses)
	b.progresses = []model.StreamProgress{}
	return err
}

// run flushes the progress buffer every interval
func (b *progressBuffer) run() {
	for {
		time.Sleep(b.interval)
		err := b.flush()
		if err != nil {
			log.WithError(err).Error("Error flushing progress buffer")
		}
	}
}

// configProgressRouter sets up the router and initializes a progress buffer
// that is used to minimize writes to the database.
func configProgressRouter(router *gin.Engine, daoWrapper dao.DaoWrapper) {
	routes := progressRoutes{daoWrapper}
	progressBuff = newProgressBuffer()
	go progressBuff.run()
	router.POST("/api/progressReport", routes.saveProgress)
	router.POST("/api/watched", routes.markWatched)
	router.GET("/api/progress/streams/:id", routes.getProgressForStream)
}

// progressRoutes contains a DaoWrapper object and all route functions dangle from it.
type progressRoutes struct {
	dao.DaoWrapper
}

// progressRequest corresponds the request that is sent by the video player when it reports its progress for VODs
type progressRequest struct {
	StreamID uint    `json:"streamID"`
	Progress float64 `json:"progress"` // A fraction that represents currentTime / totalTime for a given video
	// Note: To be able to save the progress, we also need the userID, but it`s already contained in the Gin context
}

// saveProgress saves progress to a buffer that is flushed at a fixed interval.
func (r progressRoutes) saveProgress(c *gin.Context) {
	var request progressRequest
	err := c.BindJSON(&request)

	if err != nil {
		log.WithError(err).Warn("Could not bind JSON.")
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "can not bind body",
			Err:           err,
		})
		return
	}
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "context should exist but doesn't",
		})
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	if tumLiveContext.User == nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusForbidden,
			CustomMessage: "not logged in",
		})
		return
	}
	progressBuff.add(model.StreamProgress{
		Progress: request.Progress,
		StreamID: request.StreamID,
		UserID:   tumLiveContext.User.ID,
	})
}

// watchedRequest corresponds the request that is sent when a user marked the video as watched on the watch page.
type watchedRequest struct {
	StreamID uint `json:"streamID"`
	Watched  bool `json:"watched"`
}

// markWatched marks a VoD as watched in the database.
func (r progressRoutes) markWatched(c *gin.Context) {
	var request watchedRequest
	err := c.BindJSON(&request)
	if err != nil {
		log.WithError(err).Error("Could not bind JSON.")
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "can not bind body",
			Err:           err,
		})
		return
	}
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "context should exist but doesn't",
		})
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	if tumLiveContext.User == nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusForbidden,
			CustomMessage: "not logged in",
		})
		return
	}
	prog := model.StreamProgress{
		UserID:   tumLiveContext.User.ID,
		StreamID: request.StreamID,
		Watched:  request.Watched,
	}
	err = r.ProgressDao.SaveWatchedState(&prog)
	if err != nil {
		log.WithError(err).Error("can not mark VoD as watched.")
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not mark VoD as watched.",
			Err:           err,
		})
		return
	}
}

func (r progressRoutes) getProgressForStream(c *gin.Context) {
	tumLiveContext := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)

	if tumLiveContext.User == nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusForbidden,
			CustomMessage: "Not logged-in",
		})
		return
	}

	type Stream struct {
		ID uint `uri:"id"`
	}

	stream := Stream{}

	if err := c.ShouldBindUri(&stream); err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "invalid stream id",
		})
		return
	}

	progress, err := r.LoadProgress(tumLiveContext.User.ID, stream.ID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusOK, model.StreamProgress{StreamID: stream.ID})
		} else {
			sentry.CaptureException(err)
			_ = c.Error(tools.RequestError{
				Err:           err,
				Status:        http.StatusInternalServerError,
				CustomMessage: "can't retrieve progress for user",
			})
		}
		return
	}

	c.JSON(http.StatusOK, progress)
}
