package api

import (
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	log "github.com/sirupsen/logrus"
)

func configMaintenanceRouter(router *gin.Engine, daoWrapper dao.DaoWrapper) {
	routes := maintenanceRoutes{daoWrapper}
	g := router.Group("/api/maintenance")
	g.Use(tools.Admin)
	{
		g.POST("/generateThumbnails", routes.generateThumbnails)
	}
}

type maintenanceRoutes struct {
	dao.DaoWrapper
}

func (r maintenanceRoutes) generateThumbnails(c *gin.Context) {
	courses, err := r.GetAllCourses()
	if err != nil {
		log.WithError(err).Error("Can't get courses")
	}
	// Iterate over all courses. Some might already have a valid thumbnail.
	var streams []model.Stream
	for _, course := range courses {
		streams = append(streams, course.Streams...)
	}
	for _, stream := range streams {
		for _, file := range stream.Files {
			if file.Type == model.FILETYPE_VOD {
				// Request thumbnail for VoD
				err := RegenerateThumbs(dao.DaoWrapper{}, file.Path)
				if err != nil {
					log.WithError(err).Errorf("Can't regenerate thumbnail for stream %d with file %s", stream.ID, file.Path)
					continue
				}
			}
		}
	}
}
