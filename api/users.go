package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func configGinUsersRouter(router *gin.Engine, daoWrapper dao.DaoWrapper) {
	routes := usersRoutes{daoWrapper}

	router.POST("/api/users/settings/name", routes.updatePreferredName)
	router.POST("/api/users/settings/greeting", routes.updatePreferredGreeting)
	router.POST("/api/users/settings/playbackSpeeds", routes.updatePlaybackSpeeds)

	courses := router.Group("/api/users/courses")
	{
		courses.GET("/:id/pin", routes.getPinForCourse)
		courses.POST("/pin", routes.pinCourse(true))
		courses.POST("/unpin", routes.pinCourse(false))
	}

	router.GET("/api/users/exportData", routes.exportPersonalData)

	router.POST("/api/users/init", routes.InitUser)

	admins := router.Group("/api")
	admins.Use(tools.Admin)
	admins.POST("/createUser", routes.CreateUser)
	admins.POST("/deleteUser", routes.DeleteUser)
	admins.GET("/searchUser", routes.SearchUser)
	admins.POST("/users/update", routes.updateUser)
	admins.POST("/users/impersonate", routes.impersonateUser)

	lecturers := router.Group("/api")
	lecturers.Use(tools.AtLeastLecturer)
	lecturers.GET("/searchUserForCourse", routes.SearchUserForCourse)

	courseAdmins := router.Group("/api/course/:courseID")
	courseAdmins.Use(tools.InitCourse(daoWrapper))
	courseAdmins.Use(tools.AdminOfCourse)
	courseAdmins.POST("/createUserForCourse", routes.CreateUserForCourse)
}

type usersRoutes struct {
	dao.DaoWrapper
}

func (r usersRoutes) impersonateUser(c *gin.Context) {
	type req struct {
		UserID uint `json:"id"`
	}
	var request req
	err := c.Bind(&request)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "Bad Request",
			Err:           err,
		})
		return
	}
	u, err := r.UsersDao.GetUserByID(c, request.UserID)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusNotFound,
			CustomMessage: "User not found",
			Err:           err,
		})
		return
	}
	tools.StartSession(c, &tools.SessionData{Userid: u.ID})
}

func (r usersRoutes) updateUser(c *gin.Context) {
	var req = struct {
		ID   uint `json:"id"`
		Role uint `json:"role"`
	}{}
	if err := c.BindJSON(&req); err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "can not bind body",
			Err:           err,
		})
		return
	}
	user, err := r.UsersDao.GetUserByID(c, req.ID)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not get user by id",
			Err:           err,
		})
		return
	}
	user.Role = req.Role
	err = r.UsersDao.UpdateUser(user)
	if err != nil {
		log.WithError(err).Error("can not update user")
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not update user",
			Err:           err,
		})
		return
	}
}

func (r usersRoutes) prepareUserSearch(c *gin.Context) (users []model.User, err error) {
	q := c.Query("q")
	reg, _ := regexp.Compile("[^a-zA-Z0-9 ]+")
	q = reg.ReplaceAllString(q, "")
	if len(q) < 3 {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "query too short (minimum length is 3)",
		})
		return nil, errors.New("query too short (minimum length is 3)")
	}
	users, err = r.UsersDao.SearchUser(q)
	if err != nil && err != gorm.ErrRecordNotFound {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not search user",
			Err:           err,
		})
		return nil, err
	}
	return users, nil
}

func (r usersRoutes) SearchUserForCourse(c *gin.Context) {
	users, err := r.prepareUserSearch(c)
	if err != nil {
		return
	}
	res := make([]userForLecturerDto, len(users))

	for i, user := range users {
		res[i] = userForLecturerDto{
			ID:       user.ID,
			Name:     user.GetPreferredName(),
			LastName: user.LastName,
			Login:    user.GetLoginString(),
		}
	}
	c.JSON(http.StatusOK, res)
}

func (r usersRoutes) SearchUser(c *gin.Context) {
	users, err := r.prepareUserSearch(c)
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
			Name:  user.GetPreferredName(),
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

func (r usersRoutes) DeleteUser(c *gin.Context) {
	var deleteRequest deleteUserRequest
	err := json.NewDecoder(c.Request.Body).Decode(&deleteRequest)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "can not bind body",
			Err:           err,
		})
		return
	}
	// currently admins can not be deleted.
	res, err := r.UsersDao.IsUserAdmin(context.Background(), deleteRequest.Id)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not find user",
			Err:           err,
		})
		return
	}
	if res {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "user is admin (admins can not be deleted)",
		})
		return
	}

	err = r.UsersDao.DeleteUser(context.Background(), deleteRequest.Id)
	if err != nil {
		sentry.CaptureException(err)
		defer sentry.Flush(time.Second * 2)
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not delete user",
			Err:           err,
		})
		return
	}
	c.Status(http.StatusOK)
}

func (r usersRoutes) CreateUserForCourse(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "context should exist but doesn't",
		})
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	batchUsers := c.PostForm("batchUserInput")
	userName := c.PostForm("newUserFirstName")
	userEmail := c.PostForm("newUserEmail")

	if batchUsers != "" {
		go r.addUserBatchToCourse(batchUsers, *tumLiveContext.Course)
		c.Redirect(http.StatusFound, fmt.Sprintf("/admin/course/%v", tumLiveContext.Course.ID))
		return
	} else if userName != "" && userEmail != "" {
		r.addSingleUserToCourse(userName, userEmail, *tumLiveContext.Course)
		c.Redirect(http.StatusFound, fmt.Sprintf("/admin/course/%v", tumLiveContext.Course.ID))
		return
	} else {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "invalid form",
		})
		return
	}

}

func (r usersRoutes) addUserBatchToCourse(users string, course model.Course) {
	lines := strings.Split(users, "\n")
	for _, userLine := range lines {
		userArr := strings.Split(userLine, ",")
		if len(userArr) != 2 {
			continue
		}
		r.addSingleUserToCourse(userArr[0], strings.TrimSpace(userArr[1]), course)
		time.Sleep(time.Second * 2) // send at most one email per two seconds to prevent spam blocking.
	}
}

func (r usersRoutes) addSingleUserToCourse(name string, email string, course model.Course) {
	if foundUser, err := r.UsersDao.GetUserByEmail(context.Background(), email); err != nil {
		// user not in database yet. Create them & send registration link
		createdUser := model.User{
			Name:     name,
			Email:    sql.NullString{String: email, Valid: true},
			Role:     model.GenericType,
			Password: "",
			Courses:  []model.Course{course},
		}
		if err = r.UsersDao.CreateUser(context.Background(), &createdUser); err != nil {
			log.Printf("%v", err)
		} else {
			go r.forgotPassword(email)
		}
	} else {
		// user Found, append the new course and notify via mail.
		foundUser.Courses = append(foundUser.Courses, course)
		err := r.UsersDao.UpdateUser(foundUser)
		if err != nil {
			log.WithError(err).Error("Can't update user")
			return
		}
		err = r.EmailDao.Create(context.Background(), &model.Email{
			From:    tools.Cfg.Mail.Sender,
			To:      email,
			Subject: "Setup your TUM-Live account",
			Body: fmt.Sprintf("Hello!\n"+
				"You have been invited to participate in the course \"%s\" on TUM-Live. Check it out at https://live.rbg.tum.de/",
				course.Name),
		})
		if err != nil {
			log.Printf("%v", err)
		}
	}
}

func (r usersRoutes) getPinForCourse(c *gin.Context) {
	type URI struct {
		CourseId uint `uri:"id" binding:"required"`
	}

	var uri URI
	if err := c.ShouldBindUri(&uri); err != nil {
		_ = c.Error(tools.RequestError{
			Err:           err,
			Status:        http.StatusBadRequest,
			CustomMessage: "invalid URI",
		})
		return
	}

	tumLiveContext := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)

	var has = false
	var err error
	if tumLiveContext.User != nil {
		has, err = r.UsersDao.HasPinnedCourse(*tumLiveContext.User, uri.CourseId)
		if err != nil {
			sentry.CaptureException(err)
			_ = c.Error(tools.RequestError{
				Err:           err,
				Status:        http.StatusInternalServerError,
				CustomMessage: "can't retrieve course",
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"has": has})
}

func (r usersRoutes) pinCourse(pin bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request struct {
			CourseID uint `json:"courseID"`
		}
		err := c.BindJSON(&request)
		if err != nil {
			log.WithError(err).Error("Could not bind JSON.")
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		foundContext, exists := c.Get("TUMLiveContext")
		if !exists {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		tumLiveContext := foundContext.(tools.TUMLiveContext)
		if tumLiveContext.User == nil {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		// Find course
		course, err := r.CoursesDao.GetCourseById(context.Background(), request.CourseID)
		if err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		// Update user in database
		err = r.UsersDao.PinCourse(*tumLiveContext.User, course, pin)
		if err != nil {
			log.WithError(err).Error("Can't update user")
			return
		}
	}
}

func (r usersRoutes) InitUser(c *gin.Context) {
	usersEmpty, err := r.UsersDao.AreUsersEmpty(context.Background())
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not find users",
			Err:           err,
		})
		return
	}
	if !usersEmpty {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "There are already users in the database. Use /api/createUsers instead.",
		})
		return
	}
	var request createUserRequest
	err = json.NewDecoder(c.Request.Body).Decode(&request)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "can not bind body",
			Err:           err,
		})
		return
	}

	createdUser, err := r.createUserHelper(request, model.AdminType)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not create user",
			Err:           err,
		})
		return
	}
	c.JSON(http.StatusOK, createUserResponse{Name: createdUser.Name, Email: createdUser.Email.String, Role: createdUser.Role})
}

func (r usersRoutes) CreateUser(c *gin.Context) {
	usersEmpty, err := r.UsersDao.AreUsersEmpty(context.Background())
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not find users",
			Err:           err,
		})
		return
	}
	if usersEmpty {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "No users in database. Use /api/users/init instead.",
		})
		return
	}
	var request createUserRequest
	err = json.NewDecoder(c.Request.Body).Decode(&request)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "can not bind body",
			Err:           err,
		})
		return
	}

	createdUser, err := r.createUserHelper(request, model.LecturerType)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not create user",
			Err:           err,
		})
		return
	}
	c.JSON(http.StatusOK, createUserResponse{Name: createdUser.Name, Email: createdUser.Email.String, Role: createdUser.Role})
}

func (r usersRoutes) createUserHelper(request createUserRequest, userType uint) (user model.User, err error) {
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
	dbErr := r.UsersDao.CreateUser(context.Background(), &u)
	if dbErr != nil {
		return u, errors.New("user could not be created")
	}
	if userType != model.AdminType { //generate password set link and send out email
		go r.forgotPassword(request.Email)
	}
	return u, nil
}

func (r usersRoutes) forgotPassword(email string) {
	u, err := r.UsersDao.GetUserByEmail(context.Background(), email)
	if err != nil {
		log.Println("couldn't get user by email")
		return
	}
	registerLink, err := r.UsersDao.CreateRegisterLink(context.Background(), u)
	if err != nil {
		log.Println("couldn't create register link")
		return
	}
	body := fmt.Sprintf("Hello!\n"+
		"You have been invited to use TUM-Live. You can set a password for your account here: https://live.rbg.tum.de/setPassword/%v\n"+
		"After setting a password you can log in with the email this message was sent to. Please note that this is not your TUMOnline account.\n"+
		"If you have any further questions please reach out to "+tools.Cfg.Mail.Sender, registerLink.RegisterSecret)
	err = r.EmailDao.Create(context.Background(), &model.Email{
		From:    tools.Cfg.Mail.Sender,
		To:      email,
		Subject: "Setup your TUM-Live account",
		Body:    body,
	})
	if err != nil {
		log.Println("couldn't send password mail")
	}
}

type userSettingsRequest struct {
	Value string `json:"value"`
}

func (r usersRoutes) updatePreferredName(c *gin.Context) {
	u := c.MustGet("TUMLiveContext").(tools.TUMLiveContext).User
	if u == nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusUnauthorized,
			CustomMessage: "login required",
		})
		return
	}
	var request userSettingsRequest
	err := json.NewDecoder(c.Request.Body).Decode(&request)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "can not bind body",
			Err:           err,
		})
		return
	}
	for _, s := range u.Settings {
		if s.Type == model.PreferredName && time.Since(s.UpdatedAt) < time.Hour*24*30*3 {
			_ = c.Error(tools.RequestError{
				Status:        http.StatusUnauthorized,
				CustomMessage: "preferred name already set within the last 3 months",
			})
			return
		}
	}
	if len(request.Value) > 80 {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "preferred name too long",
		})
		return
	}
	err = r.UsersDao.AddUserSetting(&model.UserSetting{
		UserID: u.ID,
		Type:   model.PreferredName,
		Value:  request.Value,
	})
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not add user setting",
			Err:           err,
		})
		return
	}
}

func (r usersRoutes) updatePreferredGreeting(c *gin.Context) {
	u := c.MustGet("TUMLiveContext").(tools.TUMLiveContext).User
	if u == nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusUnauthorized,
			CustomMessage: "login required",
		})
		return
	}
	var request userSettingsRequest
	err := json.NewDecoder(c.Request.Body).Decode(&request)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "can not bind body",
			Err:           err,
		})
		return
	}
	err = r.UsersDao.AddUserSetting(&model.UserSetting{
		UserID: u.ID,
		Type:   model.Greeting,
		Value:  request.Value,
	})
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not add user setting",
			Err:           err,
		})
		return
	}
}

func (r usersRoutes) updatePlaybackSpeeds(c *gin.Context) {
	u := c.MustGet("TUMLiveContext").(tools.TUMLiveContext).User
	if u == nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusUnauthorized,
			CustomMessage: "login required",
		})
		return
	}
	var req struct{ Value []model.PlaybackSpeedSetting }
	if err := c.BindJSON(&req); err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "can not bind body",
			Err:           err,
		})
		return
	}
	if len(req.Value) == 0 {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "invalid value (value < 1)",
		})
		return
	}
	settingBytes, _ := json.Marshal(req.Value)
	err := r.DaoWrapper.UsersDao.AddUserSetting(&model.UserSetting{UserID: u.ID, Type: model.CustomPlaybackSpeeds, Value: string(settingBytes)})
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not add user setting",
			Err:           err,
		})
		return
	}
}

func (r usersRoutes) exportPersonalData(c *gin.Context) {
	var resp personalData
	u := c.MustGet("TUMLiveContext").(tools.TUMLiveContext).User
	resp.UserData = struct {
		Name      string    `json:"name,omitempty"`
		LastName  *string   `json:"last_name,omitempty"`
		Email     string    `json:"email,omitempty"`
		LrzID     string    `json:"lrz_id,omitempty"`
		MatrNr    string    `json:"matr_nr,omitempty"`
		CreatedAt time.Time `json:"created_at"`
	}{Name: u.Name, LastName: u.LastName, Email: u.Email.String, LrzID: u.LrzID, MatrNr: u.MatriculationNumber, CreatedAt: u.CreatedAt}
	for _, course := range u.Courses {
		resp.Enrollments = append(resp.Enrollments, struct {
			Year   int    `json:"year,omitempty"`
			Term   string `json:"term,omitempty"`
			Course string `json:"course,omitempty"`
		}{course.Year, course.TeachingTerm, course.Name})
	}
	progresses, err := r.ProgressDao.GetProgressesForUser(u.ID)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not get progresses for user",
			Err:           err,
		})
		return
	}
	for _, progress := range progresses {
		resp.VideoViews = append(resp.VideoViews, struct {
			StreamID       uint    `json:"stream_id"`
			Progress       float64 `json:"progress"`
			MarkedFinished bool    `json:"marked_finished,omitempty"`
		}{StreamID: progress.StreamID, Progress: progress.Progress, MarkedFinished: progress.Watched})
	}
	chats, err := r.ChatDao.GetChatsByUser(u.ID)
	if err != nil {
		chats = []model.Chat{}
	}
	for _, chat := range chats {
		resp.Chats = append(resp.Chats, struct {
			StreamId  uint      `json:"stream_id,omitempty"`
			Message   string    `json:"message,omitempty"`
			CreatedAt time.Time `json:"created_at"`
		}{chat.StreamID, chat.Message, chat.CreatedAt})
	}
	c.Header("Content-Disposition:", `attachment; filename="personal_data.json"`)
	c.Header("Content-Type", "application/json;charset=utf-8")
	marshal, err := json.MarshalIndent(resp, "", "    ")
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not marshal response",
			Err:           err,
		})
		return
	}
	_, _ = c.Writer.Write(marshal)
}

type personalData struct {
	UserData struct {
		Name      string    `json:"name,omitempty"`
		LastName  *string   `json:"last_name,omitempty"`
		Email     string    `json:"email,omitempty"`
		LrzID     string    `json:"lrz_id,omitempty"`
		MatrNr    string    `json:"matr_nr,omitempty"`
		CreatedAt time.Time `json:"created_at"`
	} `json:"user_data"`
	Enrollments []struct {
		Year   int    `json:"year,omitempty"`
		Term   string `json:"term,omitempty"`
		Course string `json:"course,omitempty"`
	} `json:"enrollments,omitempty"`
	VideoViews []struct {
		StreamID       uint    `json:"stream_id"`
		Progress       float64 `json:"progress"`
		MarkedFinished bool    `json:"marked_finished,omitempty"`
	} `json:"video_views,omitempty"`
	Chats []struct {
		StreamId  uint      `json:"stream_id,omitempty"`
		Message   string    `json:"message,omitempty"`
		CreatedAt time.Time `json:"created_at"`
	} `json:"chats,omitempty"`
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
