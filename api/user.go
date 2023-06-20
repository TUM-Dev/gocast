package api

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"net/http"
)

func configUserRouter(r *gin.Engine, daoWrapper dao.DaoWrapper) {
	routes := userRoutes{daoWrapper}
	g := r.Group("/api/user")
	g.POST("/resetPassword", routes.resetPassword)
}

type userRoutes struct {
	dao.DaoWrapper
}

func (r userRoutes) resetPassword(c *gin.Context) {
	type resetPasswordRequest struct {
		Username string `json:"username"`
	}
	var req resetPasswordRequest
	err := c.BindJSON(&req)
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "Can't bind request body",
			Err:           err,
		})
		return
	}

	// continue in goroutine to prevent timing attacks
	go func() {
		user, err := r.UsersDao.GetUserByEmail(c, req.Username)
		if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
			// wrong username/email -> pass
			return
		}
		if err != nil {
			log.WithError(err).Error("can't get user for password reset")
			return
		}
		link, err := r.UsersDao.CreateRegisterLink(c, user)
		if err != nil {
			log.WithError(err).Error("can't create register link")
			return
		}
		err = r.EmailDao.Create(c, &model.Email{
			From:    tools.Cfg.Mail.Sender,
			To:      user.Email.String,
			Subject: "TUM-Live: Reset Password",
			Body:    "Hi! \n\nYou can reset your TUM-Live password by clicking on the following link: \n\n" + tools.Cfg.WebUrl + "/setPassword/" + link.RegisterSecret + "\n\nIf you did not request a password reset, please ignore this email. \n\nBest regards",
		})
		if err != nil {
			log.WithError(err).Error("can't save reset password email")
		}
	}()
}
