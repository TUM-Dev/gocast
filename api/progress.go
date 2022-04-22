package api

import (
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
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
	err := dao.SaveProgresses(b.progresses)
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
func configProgressRouter(router *gin.Engine) {
	progressBuff = newProgressBuffer()
	go progressBuff.run()
	router.POST("/api/progressReport", saveProgress)
	router.POST("/api/watched", markWatched)
}

// progressRequest corresponds the request that is sent by the video player when it reports its progress for VODs
type progressRequest struct {
	StreamID uint    `json:"streamID"`
	Progress float64 `json:"progress"` // A fraction that represents currentTime / totalTime for a given video
	// Note: To be able to save the progress, we also need the userID, but it`s already contained in the Gin context
}

// saveProgress saves progress to a buffer that is flushed at a fixed interval.
func saveProgress(c *gin.Context) {
	var request progressRequest
	err := c.BindJSON(&request)

	if err != nil {
		log.WithError(err).Warn("Could not bind JSON.")
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	if tumLiveContext.User == nil {
		c.AbortWithStatus(http.StatusForbidden)
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
func markWatched(c *gin.Context) {
	var request watchedRequest
	err := c.BindJSON(&request)
	if err != nil {
		log.WithError(err).Error("Could not bind JSON.")
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	if tumLiveContext.User == nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	progress := model.StreamProgress{
		UserID:   tumLiveContext.User.ID,
		StreamID: request.StreamID,
		Watched:  request.Watched,
	}
	err = dao.SaveProgresses([]model.StreamProgress{progress})
	if err != nil {
		log.WithError(err).Error("Could not mark VoD as watched.")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
}
