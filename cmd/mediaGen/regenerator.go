package mediaGen

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/api"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	"github.com/joschahenningsen/TUM-Live/web"
	log "github.com/sirupsen/logrus"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// This simple program is used to populate TUM-Live with thumbnails for all VoDs.

func main() {
	// Establish database connection.
	db, err := gorm.Open(mysql.Open(fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?parseTime=true&loc=Local",
		tools.Cfg.Db.User,
		tools.Cfg.Db.Password,
		tools.Cfg.Db.Host,
		tools.Cfg.Db.Port,
		tools.Cfg.Db.Database),
	))

	if err != nil {
		log.WithError(err).Fatal("Could not connect to database")
	}

	dao.DB = db

	if err != nil {
		log.WithError(err).Error("Can't get courses")
	}

	router := gin.Default()
	router.Use(tools.InitContext(dao.NewDaoWrapper()))
	api.ConfigGinRouter(router)
	web.ConfigGinRouter(router)

	r := dao.NewCoursesDao()
	courses, err := r.GetAllCourses()

	if err != nil {
		log.WithError(err).Error("Can't get courses")
	}
	// Iterate over all courses. Some might already have a valid thumbnail.
	for _, course := range courses {
		for _, stream := range course.Streams {
			for _, file := range stream.Files {
				if file.Type == model.FILETYPE_VOD {
					// Request thumbnail for VoD
					err := api.RegenerateThumbs(dao.DaoWrapper{}, file.Path)
					if err != nil {
						log.WithError(err).Errorf("Can't regenerate thumbnail for stream %d with file %s", stream.ID, file.Path)
						continue
					}
				}
			}
		}
		log.Info("Regenerated thumbnails for course ", course.ID)
	}
}
