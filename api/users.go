package api

import (
	"TUM-Live/dao"
	"TUM-Live/model"
	"TUM-Live/tools"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"net/http"
	"regexp"
	"strings"
	"time"
)

func configGinUsersRouter(router *gin.Engine) {
	admins := router.Group("/api")
	admins.Use(tools.Admin)
	admins.POST("/createUser", CreateUser)
	admins.POST("/deleteUser", DeleteUser)
	admins.GET("/searchUser", SearchUser)
	admins.POST("/users/update", updateUser)

	lecturers := router.Group("/api")
	lecturers.Use(tools.AtLeastLecturer)
	lecturers.GET("/searchUserForCourse", SearchUserForCourse)

	courseAdmins := router.Group("/api/course/:courseID")
	courseAdmins.Use(tools.InitCourse)
	courseAdmins.Use(tools.AdminOfCourse)
	courseAdmins.POST("/createUserForCourse", CreateUserForCourse)
}

func updateUser(c *gin.Context) {
	var req = struct {
		ID   uint `json:"id"`
		Role uint `json:"role"`
	}{}
	if err := c.BindJSON(&req); err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	user, err := dao.GetUserByID(c, req.ID)
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	user.Role = req.Role
	err = dao.UpdateUser(user)
	if err != nil {
		log.WithError(err).Error("Error while updating user")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
}

func prepareUserSearch(c *gin.Context) (users []model.User, err error) {
	q := c.Query("q")
	reg, _ := regexp.Compile("[^a-zA-Z0-9 ]+")
	q = reg.ReplaceAllString(q, "")
	if len(q) < 3 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query too short"})
		return nil, errors.New("query too short")
	}
	users, err = dao.SearchUser(q)
	if err != nil && err != gorm.ErrRecordNotFound {
		c.AbortWithStatus(http.StatusInternalServerError)
		return nil, err
	}
	return users, nil
}

func SearchUserForCourse(c *gin.Context) {
	users, err := prepareUserSearch(c)
	if err != nil {
		return
	}
	res := make([]userForLecturerDto, len(users))

	for i, user := range users {
		res[i] = userForLecturerDto{
			ID:       user.ID,
			Name:     user.Name,
			LastName: user.LastName,
			Login:    user.GetLoginString(),
		}
	}
	c.JSON(http.StatusOK, res)
}

func SearchUser(c *gin.Context) {
	users, err := prepareUserSearch(c)
	if err != nil {
		return
	}
	res := make([]userSearchDTO, len(users))
	for i, user := range users {
		email, err := tools.MaskEmail(user.Email.String)
		if err != nil {
			email = ""
		}
		lrzID := tools.MaskLogin(user.LrzID)
		res[i] = userSearchDTO{
			ID:    user.ID,
			LrzID: lrzID,
			Email: email,
			Name:  user.Name,
			Role:  user.Role,
		}
	}
	c.JSON(http.StatusOK, res)
}

type userForLecturerDto struct {
	ID       uint    `json:"id,omitempty"`
	Name     string  `json:"name,omitempty"`
	LastName *string `json:"lastName,omitempty"`
	Login    string  `json:"login,omitempty"`
}

type userSearchDTO struct {
	ID    uint   `json:"id"`
	LrzID string `json:"lrz_id"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  uint   `json:"role"`

	// used by alpine
	Changing bool `json:"changing"`
}

func DeleteUser(c *gin.Context) {
	var deleteRequest deleteUserRequest
	err := json.NewDecoder(c.Request.Body).Decode(&deleteRequest)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
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
		sentry.CaptureException(err)
		defer sentry.Flush(time.Second * 2)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.Status(http.StatusOK)
}

func CreateUserForCourse(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	batchUsers := c.PostForm("batchUserInput")
	userName := c.PostForm("newUserFirstName")
	userEmail := c.PostForm("newUserEmail")

	if batchUsers != "" {
		go addUserBatchToCourse(batchUsers, *tumLiveContext.Course)
		c.Redirect(http.StatusFound, fmt.Sprintf("/admin/course/%v", tumLiveContext.Course.ID))
		return
	} else if userName != "" && userEmail != "" {
		addSingleUserToCourse(userName, userEmail, *tumLiveContext.Course)
		c.Redirect(http.StatusFound, fmt.Sprintf("/admin/course/%v", tumLiveContext.Course.ID))
		return
	} else {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"msg": "bad request"})
		return
	}

}

func addUserBatchToCourse(users string, course model.Course) {
	lines := strings.Split(users, "\n")
	for _, userLine := range lines {
		userArr := strings.Split(userLine, ",")
		if len(userArr) != 2 {
			continue
		}
		addSingleUserToCourse(userArr[0], strings.TrimSpace(userArr[1]), course)
		time.Sleep(time.Second * 2) // send at most one email per two seconds to prevent spam blocking.
	}
}

func addSingleUserToCourse(name string, email string, course model.Course) {
	if foundUser, err := dao.GetUserByEmail(context.Background(), email); err != nil {
		// user not in database yet. Create them & send registration link
		createdUser := model.User{
			Name:     name,
			Email:    sql.NullString{String: email, Valid: true},
			Role:     model.GenericType,
			Password: "",
			Courses:  []model.Course{course},
		}
		if err = dao.CreateUser(context.Background(), &createdUser); err != nil {
			log.Printf("%v", err)
		} else {
			go forgotPassword(email)
		}
	} else {
		// user Found, append the new course and notify via mail.
		foundUser.Courses = append(foundUser.Courses, course)
		err := dao.UpdateUser(foundUser)
		if err != nil {
			log.WithError(err).Error("Can't update user")
			return
		}
		err = tools.SendPasswordMail(email,
			fmt.Sprintf("Hello!\n"+
				"You have been invited to participate in the course \"%v\" on TUM-Live. Check it out at https://live.rbg.tum.de/",
				course.Name))
		if err != nil {
			log.Printf("%v", err)
		}
	}
}

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
		createdUser, err = createUserHelper(request, model.LecturerType)
	}
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, createUserResponse{Name: createdUser.Name, Email: createdUser.Email.String, Role: createdUser.Role})
}

func createUserHelper(request createUserRequest, userType uint) (user model.User, err error) {
	var u = model.User{
		Name:  request.Name,
		Email: sql.NullString{String: request.Email, Valid: true},
		Role:  userType,
	}
	if userType == 1 {
		err = u.SetPassword(request.Password)
		if err != nil {
			return u, errors.New("user could not be created")
		}
	}
	dbErr := dao.CreateUser(context.Background(), &u)
	if dbErr != nil {
		return u, errors.New("user could not be created")
	}
	if userType != model.AdminType { //generate password set link and send out email
		go forgotPassword(request.Email)
	}
	return u, nil
}

func forgotPassword(email string) {
	u, err := dao.GetUserByEmail(context.Background(), email)
	if err != nil {
		log.Println("couldn't get user by email")
		return
	}
	registerLink, err := dao.CreateRegisterLink(context.Background(), u)
	if err != nil {
		log.Println("couldn't create register link")
		return
	}
	log.Println("register link:", registerLink)
	body := fmt.Sprintf("Hello!\n"+
		"You have been invited to use TUM-Live. You can set a password for your account here: https://live.rbg.tum.de/setPassword/%v\n"+
		"After setting a password you can log in with the email this message was sent to. Please note that this is not your TUMOnline account.\n"+
		"If you have any further questions please reach out to "+tools.Cfg.Mail.Sender, registerLink.RegisterSecret)
	err = tools.SendPasswordMail(email, body)
	if err != nil {
		log.Println("couldn't send password mail")
	}
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
	Role  uint   `json:"role"`
}
