package tools

import (
	"github.com/TUM-Dev/gocast/model"
	"github.com/gin-gonic/gin"
)

// AdminCourseJson is the JSON representation of a courses streams for the admin panel
func AdminCourseJson(c *model.Course, lhs []model.LectureHall, u *model.User) []gin.H {
	var res []gin.H
	streams := c.Streams
	for _, s := range streams {
		err := SetSignedPlaylists(&s, u, true)
		if err != nil {
			logger.Error("Could not sign playlist for admin", "err", err)
		}
		res = append(res, s.GetJson(lhs, *c))
	}
	return res
}
