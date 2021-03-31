package api

import (
	"TUM-Live/dao"
	"TUM-Live/model"
	"TUM-Live/tools"
	"context"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
)

func configGinUsersRouter(router gin.IRoutes) {
	router.POST("/api/createUser", CreateUser)
	router.POST("/api/deleteUser", DeleteUser)
}

func DeleteUser(c *gin.Context) {
	var deleteRequest deleteUserRequest
	err := json.NewDecoder(c.Request.Body).Decode(&deleteRequest)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	err = tools.RequirePermission(c, 1) // require admin
	if err != nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	// currently admins can not be deleted.
	res, err := dao.IsUserAdmin(context.Background(), deleteRequest.Id)
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	if res {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	err = dao.DeleteUser(context.Background(), deleteRequest.Id)
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.Status(200)
}

// todo: refactor
func CreateUser(c *gin.Context) {
	usersEmpty, err := dao.AreUsersEmpty(context.Background())
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	var request createUserRequest
	err = json.NewDecoder(c.Request.Body).Decode(&request)
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	var createdUser model.User
	if usersEmpty {
		createdUser, err = createUserHelper(request, model.AdminType)
	} else {
		requestUser, err := tools.GetUser(c)
		if err != nil || requestUser.Role > 1 {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		createdUser, err = createUserHelper(request, model.LecturerType)
	}
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.JSON(200, createUserResponse{Name: createdUser.Name, Email: createdUser.Email, Role: createdUser.Role})
}

func createUserHelper(request createUserRequest, userType int) (user model.User, err error) {
	var u = model.User{
		Name:  request.Name,
		Email: request.Email,
		Role:  userType,
	}
	if userType == 1 {
		err = u.SetPassword(request.Password)
		if err != nil {
			return u, errors.New("user could not be created")
		}
	}
	if !u.ValidateFields() {
		return u, errors.New("user data rejected")
	}
	dbErr := dao.CreateUser(context.Background(), u)
	if dbErr != nil {
		return u, errors.New("user could not be created")
	}
	if userType != model.AdminType { //generate password set link and send out email
		if err = forgotPassword(request.Email); err != nil {
			log.Printf("error sending email: %v\n", err)
		}
	}
	return u, nil
}

func forgotPassword(email string) error {
	u, err := dao.GetUserByEmail(context.Background(), email)
	if err != nil {
		log.Printf("couldn't get user by email")
		return err
	}
	registerLink, err := dao.CreateRegisterLink(context.Background(), u)
	if err != nil {
		log.Printf("couldn't create register link\n")
		return err
	}
	log.Printf("register link: %v\n", registerLink)
	err = tools.SendPasswordMail(email, registerLink.RegisterSecret)
	return err
}

type deleteUserRequest struct {
	Id uint `json:"id"`
}

type createUserRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type createUserResponse struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  int    `json:"role"`
}
