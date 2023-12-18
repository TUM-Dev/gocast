package api

import (
	"fmt"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

func configMaintenanceRouter(router *gin.Engine, daoWrapper dao.DaoWrapper) {
	routes := maintenanceRoutes{DaoWrapper: daoWrapper}

	g := router.Group("/api/maintenance")
	g.Use(tools.Admin)
	{
		g.POST("/generateThumbnails", routes.generateThumbnails)
		g.GET("/generateThumbnails/status", routes.getThumbGenProgress)

		g.GET("/transcodingFailures", routes.getTranscodingFailures)
		g.DELETE("/transcodingFailures/:id", routes.deleteTranscodingFailure)

		g.GET("/emailFailures", routes.getEmailFailures)
		g.DELETE("/emailFailures/:id", routes.deleteEmailFailure)
	}

	cronGroup := g.Group("/cron")
	{
		cronGroup.GET("/available", routes.listCronJobs)
		cronGroup.POST("/run", routes.runCronJob)
	}
}

type maintenanceRoutes struct {
	dao.DaoWrapper

	thumbGenProgress float32
	thumbGenRunning  bool
}

func (r *maintenanceRoutes) generateThumbnails(c *gin.Context) {
	noFiles, err := r.FileDao.CountVoDFiles()
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can't find number of files",
			Err:           nil,
		})
		return
	}
	logger.Info("generating " + strconv.FormatInt(noFiles, 10) + " thumbs")
	go func() {
		courses, err := r.GetAllCourses()
		if err != nil {
			logger.Error("Can't get courses", "err", err)
		}
		processed := 0
		r.thumbGenRunning = true
		defer func() {
			r.thumbGenRunning = false
			r.thumbGenProgress = 0
		}()
		// Iterate over all courses. Some might already have a valid thumbnail.
		for _, course := range courses {
			for _, stream := range course.Streams {
				for _, file := range stream.Files {
					if file.Type != model.FILETYPE_VOD {
						continue
					}
					// Request thumbnail for VoD.
					err := RegenerateThumbs(dao.NewDaoWrapper(), file, &stream, &course)
					if err != nil {
						logger.Error(fmt.Sprintf(
							"Can't regenerate thumbnail for stream %d with file %s",
							stream.ID,
							file.Path,
						))
						continue
					}
					logger.Info("Processed thumbnail" + string(rune(processed)) + "of" + strconv.FormatInt(noFiles, 10))
					processed++
					r.thumbGenProgress = float32(processed) / float32(noFiles)
				}
			}
		}
	}()
}

func (r *maintenanceRoutes) getThumbGenProgress(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"running":  r.thumbGenRunning,
		"progress": r.thumbGenProgress,
	})
}

func (r *maintenanceRoutes) listCronJobs(c *gin.Context) {
	c.JSON(http.StatusOK, tools.Cron.ListCronJobs())
}

func (r *maintenanceRoutes) runCronJob(c *gin.Context) {
	err := c.Request.ParseForm()
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "Can't read request",
			Err:           err,
		})
		return
	}
	jobName := c.Request.FormValue("job")
	logger.Info("request to run " + jobName)
	tools.Cron.RunJob(jobName)
}

func (r *maintenanceRoutes) getTranscodingFailures(c *gin.Context) {
	all, err := r.TranscodingFailureDao.All()
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "Can't get transcoding failures",
			Err:           err,
		})
		return
	}
	c.JSON(http.StatusOK, all)
}

func (r *maintenanceRoutes) deleteTranscodingFailure(c *gin.Context) {
	atoi, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "Can't parse id",
			Err:           err,
		})
		return
	}
	err = r.TranscodingFailureDao.Delete(uint(atoi))
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "Can't delete transcoding failure",
			Err:           err,
		})
	}
}

func (r *maintenanceRoutes) getEmailFailures(c *gin.Context) {
	failed, err := r.EmailDao.GetFailed(c)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "Can't get email failures",
			Err:           err,
		})
		return
	}
	c.JSON(http.StatusOK, failed)
}

func (r *maintenanceRoutes) deleteEmailFailure(c *gin.Context) {
	atoi, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "Can't parse id",
			Err:           err,
		})
		return
	}
	err = r.EmailDao.Delete(c, uint(atoi))
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "Can't delete email failure",
			Err:           err,
		})
	}
}
