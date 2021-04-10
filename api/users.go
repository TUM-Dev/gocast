package api

import (
	"TUM-Live/dao"
	"TUM-Live/model"
	"TUM-Live/tools"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func configGinUsersRouter(router gin.IRoutes) {
	router.POST("/api/createUser", CreateUser)
	router.POST("/api/createUserForCourse", CreateUserForCourse)
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

func CreateUserForCourse(c *gin.Context) {
	user, userErr := tools.GetUser(c)
	if userErr != nil {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"msg": "Not Logged in."})
		return
	}
	courseID, err := strconv.Atoi(c.PostForm("courseID"))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"msg": "bad request"})
		return
	}
	batchUsers := c.PostForm("batchUserInput")
	userName := c.PostForm("newUserFirstName")
	userEmail := c.PostForm("newUserEmail")
	if user.Role != model.AdminType && !user.IsAdminOfCourse(uint(courseID)) {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"msg": "forbidden"})
		return
	}
	course, err := dao.GetCourseById(context.Background(), uint(courseID))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"msg": "no such course."})
		return
	}

	if batchUsers != "" {
		go addUserBatchToCourse(batchUsers, course)
		c.Redirect(http.StatusFound, fmt.Sprintf("/admin/course/%v", courseID))
		return
	} else if userName != "" && userEmail != "" {
		addSingleUserToCourse(userName, userEmail, course)
		c.Redirect(http.StatusFound, fmt.Sprintf("/admin/course/%v", courseID))
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
			Name:           name,
			Email:          email,
			Role:           model.GenericType,
			Password:       "",
			Courses:        nil,
			InvitedCourses: []model.Course{course},
		}
		if err = dao.CreateUser(context.Background(), createdUser); err != nil {
			log.Printf("%v", err)
		} else {
			go forgotPassword(email)
		}
	} else {
		// user Found, append the new course and notify via mail.
		foundUser.InvitedCourses = append(foundUser.InvitedCourses, course)
		dao.UpdateUser(foundUser)
		err = tools.SendPasswordMail(email,
			fmt.Sprintf("Hello!\n"+
				"You have been invited to participate in the course \"%v\" on TUM-Live. Check it out at <a href=\"https://live.mm.rbg.tum.de/\">https://live.mm.rbg.tum.de/</a>",
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
		requestUser, err := tools.GetUser(c)
		if err != nil || requestUser.Role > model.AdminType {
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
		go forgotPassword(request.Email)
	}
	return u, nil
}

func forgotPassword(email string) {
	u, err := dao.GetUserByEmail(context.Background(), email)
	if err != nil {
		log.Printf("couldn't get user by email")
		return
	}
	registerLink, err := dao.CreateRegisterLink(context.Background(), u)
	if err != nil {
		log.Printf("couldn't create register link\n")
		return
	}
	log.Printf("register link: %v\n", registerLink)
	body := fmt.Sprintf("Hello!<br>\n"+
		"You have been invited to use TUM-Live. You can set a password for your account here: <a href=\"https://live.mm.rbg.tum.de/setPassword/%v\">https://live.mm.rbg.tum.de/setPassword/%v</a>.</br>\n"+
		"If you have any further questions please reach out to <a href=\"multimedia@rbg.in.tum.de\">multimedia@rbg.in.tum.de</a>", registerLink.RegisterSecret, registerLink.RegisterSecret)
	err = tools.SendPasswordMail(email, body)
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
