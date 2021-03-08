package tools

import (
	"TUM-Live/dao"
	"TUM-Live/model"
	"context"
	"errors"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

func GetStudent(c *gin.Context) (student model.Student, err error) {
	s := sessions.Default(c)
	sid := s.Get("StudentID")
	if sid != nil {
		return dao.GetStudent(context.Background(), sid.(string))
	}
	return model.Student{}, errors.New("not a student")
}

func GetUser(c *gin.Context) (student model.User, err error) {
	s := sessions.Default(c)
	uid := s.Get("UserID")
	if uid != nil {
		return dao.GetUserByID(context.Background(), uid.(uint))
	}
	return model.User{}, errors.New("not a user")
}

func RequirePermission(c *gin.Context, permLevel int) (err error) {
	s := sessions.Default(c)
	userid := s.Get("UserID")
	if userid == nil {
		return errors.New("not authenticated")
	}
	user, err := dao.GetUserByID(context.Background(), userid.(uint))
	if err != nil {
		return errors.New("not authenticated")
	}
	if user.Role > permLevel {
		return errors.New("insufficient permission")
	}
	return nil
}
