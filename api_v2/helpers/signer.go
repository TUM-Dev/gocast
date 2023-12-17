// Package helpers provides helper functions for parsing models to protobuf representations.
package helpers

import (
	"github.com/TUM-Dev/gocast/model"
    "github.com/TUM-Dev/gocast/tools"
	e "github.com/TUM-Dev/gocast/api_v2/errors"
	"net/http"
	"errors"
	"gorm.io/gorm"
)

func SignStream(s *model.Stream, c *model.Course, uID uint) error {
    if err := tools.SetSignedPlaylists(s, &model.User{
		Model: gorm.Model{ID: uID},
	}, c.DownloadsEnabled); err != nil {
		return e.WithStatus(http.StatusInternalServerError, errors.New("can't sign stream"))
	} else {
		return nil
	}
}
