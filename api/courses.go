package api

import (
	"TUM-Live/dao"
	"TUM-Live/model"
	"TUM-Live/tools"
	"TUM-Live/tools/tum"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	uuid "github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func configGinCourseRouter(router *gin.Engine) {
	router.GET("/api/course-by-token", courseByToken)
	router.GET("/api/lecture-halls-by-token", lectureHallsByToken)
	router.GET("/api/lecture-halls-by-id", lectureHallsByID)
	router.POST("/api/course-by-token", courseByTokenPost)
	atLeastLecturerGroup := router.Group("/")
	atLeastLecturerGroup.Use(tools.AtLeastLecturer)
	atLeastLecturerGroup.POST("/api/courseInfo", courseInfo)
	atLeastLecturerGroup.POST("/api/createCourse", createCourse)

	adminOfCourseGroup := router.Group("/api/course/:courseID")
	adminOfCourseGroup.Use(tools.InitCourse)
	adminOfCourseGroup.Use(tools.AdminOfCourse)
	adminOfCourseGroup.DELETE("/", deleteCourse)
	adminOfCourseGroup.POST("/createLecture", createLecture)
	adminOfCourseGroup.POST("/deleteLectures", deleteLectures)
	adminOfCourseGroup.POST("/renameLecture/:streamID", renameLecture)
	adminOfCourseGroup.POST("/updateDescription/:streamID", updateDescription)
	adminOfCourseGroup.POST("/addUnit", addUnit)
	adminOfCourseGroup.POST("/submitCut", submitCut)
	adminOfCourseGroup.POST("/deleteUnit/:unitID", deleteUnit)
	adminOfCourseGroup.GET("/stats", getStats)
	adminOfCourseGroup.GET("/admins", getAdmins)
	adminOfCourseGroup.PUT("/admins/:userID", addAdminToCourse)
	adminOfCourseGroup.DELETE("/admins/:userID", removeAdminFromCourse)
}

func removeAdminFromCourse(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)

	userID, err := strconv.ParseUint(c.Param("userID"), 10, 32)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	admins, err := dao.GetCourseAdmins(tumLiveContext.Course.ID)
	if err != nil {
		log.WithError(err).Error("could not get course admins")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	if len(admins) == 1 {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	var user *model.User
	for _, u := range admins {
		if u.ID == uint(userID) {
			user = &u
			break
		}
	}
	if user == nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	err = dao.RemoveAdminFromCourse(user.ID, tumLiveContext.Course.ID)
	if err != nil {
		log.WithError(err).Error("could not remove admin from course")
		c.AbortWithStatus(http.StatusInternalServerError)
	}
	c.JSON(http.StatusOK, userForLecturerDto{
		ID:    user.ID,
		Name:  user.Name,
		Login: user.GetLoginString(),
	})
}

func addAdminToCourse(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	id := c.Param("userID")
	idUint, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	user, err := dao.GetUserByID(c, uint(idUint))
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	err = dao.AddAdminToCourse(user.ID, tumLiveContext.Course.ID)
	if err != nil {
		log.WithError(err).Error("could not add admin to course")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	if user.Role == model.GenericType || user.Role == model.StudentType {
		user.Role = model.LecturerType
		err := dao.UpdateUser(user)
		if err != nil {
			log.WithError(err).Error("could not update user")
			c.AbortWithStatus(http.StatusInternalServerError)
		}
	}
	c.JSON(http.StatusOK, userForLecturerDto{
		ID:    user.ID,
		Name:  user.Name,
		Login: user.GetLoginString(),
	})
}

func getAdmins(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	admins, err := dao.GetCourseAdmins(tumLiveContext.Course.ID)
	if err != nil {
		log.WithError(err).Error("error getting course admins")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	res := make([]userForLecturerDto, len(admins))
	for i, admin := range admins {
		res[i] = userForLecturerDto{
			ID:    admin.ID,
			Name:  admin.Name,
			Login: admin.GetLoginString(),
		}
	}
	c.JSON(http.StatusOK, res)
}

type courseByTokenReq struct {
	AdminEmail string       `json:"adminEmail"`
	AdminName  string       `json:"adminName"`
	Course     model.Course `json:"course"`
	Halls      []lhResp     `json:"halls"`
	Token      string       `json:"token"`
}

func courseByTokenPost(c *gin.Context) {
	var req courseByTokenReq
	err := c.BindJSON(&req)
	if err != nil {
		return
	}
	course, err := dao.GetCourseByToken(req.Token)
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	if !(req.Course.VODEnabled || req.Course.LiveEnabled) {
		dao.DeleteCourse(course)
		return
	} else {
		course.DeletedAt = gorm.DeletedAt{
			Time:  time.Now(),
			Valid: false,
		}
	}

	if req.AdminEmail != "" && !course.UserCreatedByToken {
		var user model.User
		user, err = dao.GetUserByEmail(c, req.AdminEmail)
		if err != nil {
			user, err = createUserHelper(createUserRequest{
				Name:     req.AdminName,
				Email:    req.AdminEmail,
				Password: "",
			}, model.LecturerType)
			if err != nil {
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}
		}
		course.UserCreatedByToken = true
		course.UserID = user.ID
	}
	var presetSettings []model.CameraPresetPreference
	for _, hall := range req.Halls {
		if len(hall.Presets) != 0 && hall.SelectedIndex != 0 {
			presetSettings = append(presetSettings, model.CameraPresetPreference{
				LectureHallID: hall.Presets[hall.SelectedIndex-1].LectureHallId, // index count starts at 1
				PresetID:      hall.SelectedIndex,
			})
		}
	}
	course.Visibility = req.Course.Visibility
	course.VODEnabled = req.Course.VODEnabled
	course.LiveEnabled = req.Course.LiveEnabled
	course.ChatEnabled = req.Course.ChatEnabled
	course.VodChatEnabled = req.Course.VodChatEnabled
	course.DownloadsEnabled = req.Course.DownloadsEnabled
	course.Name = req.Course.Name

	course.SetCameraPresetPreference(presetSettings)

	err = dao.UpdateCourseSettings(c, course)
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	log.Info(c)
}

type lhResp struct {
	LectureHallName string               `json:"lecture_hall_name"`
	Presets         []model.CameraPreset `json:"presets"`
	SelectedIndex   int                  `json:"selected_index"`
}

func lectureHallsByID(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	err := c.Request.ParseForm()
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	token := c.Request.Form.Get("id")
	id, err := strconv.Atoi(token)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	course, err := dao.GetCourseById(c, uint(id))
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	if !tumLiveContext.User.IsAdminOfCourse(course) {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	lectureHalls(c, course)
}

func lectureHallsByToken(c *gin.Context) {
	err := c.Request.ParseForm()
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	token := c.Request.Form.Get("token")
	if len([]rune(token)) != 15 {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	course, err := dao.GetCourseByToken(token)
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	lectureHalls(c, course)
}

func lectureHalls(c *gin.Context, course model.Course) {
	var res []lhResp
	lectureHallIDs := map[uint]bool{}
	for _, s := range course.Streams {
		if s.LectureHallID != 0 {
			lectureHallIDs[s.LectureHallID] = true
		}
	}
	for u := range lectureHallIDs {
		lh, err := dao.GetLectureHallByID(u)
		if err != nil {
			log.WithError(err).Error("Can't fetch lecture hall for stream")
		} else {
			res = append(res, lhResp{
				LectureHallName: lh.Name,
				Presets:         lh.CameraPresets,
			})
		}
	}
	for _, preference := range course.GetCameraPresetPreference() {
		for i, re := range res {
			if len(re.Presets) != 0 && re.Presets[0].LectureHallId == preference.LectureHallID {
				res[i].SelectedIndex = preference.PresetID
				break
			}
		}
	}

	c.JSON(http.StatusOK, res)
}

func courseByToken(c *gin.Context) {
	err := c.Request.ParseForm()
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	token := c.Request.Form.Get("token")
	if len([]rune(token)) != 15 {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	course, err := dao.GetCourseByToken(token)
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	c.JSON(http.StatusOK, course)
}

func submitCut(c *gin.Context) {
	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"msg": "bad request"})
		return
	}
	var req submitCutRequest
	if err = json.Unmarshal(body, &req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"msg": "bad request"})
		return
	}
	stream, err := dao.GetStreamByID(context.Background(), strconv.Itoa(int(req.LectureID)))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"msg": "stream not found"})
		return
	}
	stream.StartOffset = req.From
	stream.EndOffset = req.To
	if err = dao.SaveStream(&stream); err != nil {
		panic(err)
	}
}

type submitCutRequest struct {
	LectureID uint `json:"lectureID"`
	From      uint `json:"from"`
	To        uint `json:"to"`
}

func deleteUnit(c *gin.Context) {
	unit, err := dao.GetUnitByID(c.Param("unitID"))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"msg": "not found"})
		return
	}
	dao.DeleteUnit(unit.Model.ID)
}

func addUnit(c *gin.Context) {
	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"msg": "bad request"})
		return
	}
	var req addUnitRequest
	if err = json.Unmarshal(body, &req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"msg": "bad request"})
		return
	}
	stream, err := dao.GetStreamByID(context.Background(), strconv.Itoa(int(req.LectureID)))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"msg": "stream not found"})
		return
	}
	stream.Units = append(stream.Units, model.StreamUnit{
		UnitName:        req.Title,
		UnitDescription: req.Description,
		UnitStart:       req.From,
		UnitEnd:         req.To,
		StreamID:        stream.Model.ID,
	})
	if err = dao.UpdateStreamFullAssoc(&stream); err != nil {
		panic(err)
	}
}

type addUnitRequest struct {
	LectureID   uint   `json:"lectureID"`
	From        uint   `json:"from"`
	To          uint   `json:"to"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

func updateDescription(c *gin.Context) {
	sIDInt, err := strconv.Atoi(c.Param("streamID"))
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	sID := uint(sIDInt)
	var req renameLectureRequest
	if err := c.Bind(&req); err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	stream, err := dao.GetStreamByID(context.Background(), c.Param("streamID"))
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	stream.Description = req.Name
	if err := dao.UpdateStream(stream); err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, "couldn't update lecture Description")
		return
	}
	wsMsg := gin.H{
		"description": stream.GetDescriptionHTML(),
	}
	if msg, err := json.Marshal(wsMsg); err == nil {
		broadcastStream(sID, msg)
	} else {
		log.WithError(err).Error("couldn't marshal stream rename ws msg")
	}
}

func renameLecture(c *gin.Context) {
	sIDInt, err := strconv.Atoi(c.Param("streamID"))
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	sID := uint(sIDInt)
	var req renameLectureRequest
	if err = c.Bind(&req); err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	stream, err := dao.GetStreamByID(context.Background(), c.Param("streamID"))
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	stream.Name = req.Name
	if err := dao.UpdateStream(stream); err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, "couldn't update lecture name")
		return
	}
	wsMsg := gin.H{
		"title": req.Name,
	}
	if msg, err := json.Marshal(wsMsg); err == nil {
		broadcastStream(sID, msg)
	} else {
		log.WithError(err).Error("couldn't marshal stream rename ws msg")
	}
}

type renameLectureRequest struct {
	Name string
}

func deleteLectures(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)

	jsonData, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	var req deleteLecturesRequest
	err = json.Unmarshal(jsonData, &req)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	var streams []model.Stream
	for _, streamID := range req.StreamIDs {
		stream, err := dao.GetStreamByID(context.Background(), streamID)
		if err != nil || stream.CourseID != tumLiveContext.Course.ID {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		streams = append(streams, stream)
	}

	for _, stream := range streams {
		dao.DeleteStream(strconv.Itoa(int(stream.ID)))
	}
}

func createLecture(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	var req createLectureRequest
	if err := c.ShouldBind(&req); err != nil {
		log.WithError(err).Error("invalid form")
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	// Forbid setting lectureHall for vod or premiere
	if (req.Premiere || req.Vodup) && req.LectureHallId != "0" {
		log.Error("Cannot set lectureHallId on 'Premiere' or 'Vodup' Lecture.")
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	// try parse lectureHallId
	lectureHallId, err := strconv.ParseInt(req.LectureHallId, 10, 32)
	if err != nil {
		log.WithError(err).Error("invalid LectureHallId format")
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	// name for folder for premiere file if needed
	premiereFolder := fmt.Sprintf("%s/%d/%s/%s",
		tools.Cfg.Paths.Mass,
		tumLiveContext.Course.Year,
		tumLiveContext.Course.TeachingTerm,
		tumLiveContext.Course.Slug)
	premiereFileName := fmt.Sprintf("%s_%s.mp4",
		tumLiveContext.Course.Slug,
		req.Start.Format("2006-01-02_15-04"))
	if req.Premiere || req.Vodup {
		err := os.MkdirAll(premiereFolder, os.ModePerm)
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			log.WithError(err).Error("Can't create folder for premiere")
			return
		}
		//// Copy file to shared storage
		//file, err := os.Create(fmt.Sprintf("%s/%s", premiereFolder, premiereFileName))
		//if err != nil {
		//	log.WithError(err).Error("Can't create file for premiere")
		//	c.AbortWithStatus(http.StatusInternalServerError)
		//	return
		//}
		//reqFile, _ := req.File.Open()
		//_, err = io.Copy(file, reqFile)
		//if err != nil {
		//	log.WithError(err).Error("Can't write file for premiere")
		//	c.AbortWithStatus(http.StatusInternalServerError)
		//}
		//_ = file.Close()
	}
	streamKey := uuid.NewV4().String()
	streamKey = strings.ReplaceAll(streamKey, "-", "")
	playlist := ""
	if req.Vodup {
		err := tools.UploadLRZ(fmt.Sprintf("%s/%s", premiereFolder, premiereFileName))
		if err != nil {
			log.WithError(err).Error("Can't upload file for premiere")
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		playlist = fmt.Sprintf("https://stream.lrz.de/vod/_definst_/mp4:tum/RBG/%s/playlist.m3u8", strings.ReplaceAll(premiereFileName, "-", "_"))
	}

	// Add start date as first event
	seriesIdentifier := uuid.NewV4().String()
	req.DateSeries = append(req.DateSeries, req.Start)

	for _, date := range req.DateSeries {
		endTime := date.Add(time.Minute * time.Duration(req.Duration))

		lecture := model.Stream{
			Name:          req.Title,
			CourseID:      tumLiveContext.Course.ID,
			LectureHallID: uint(lectureHallId),
			Start:         date,
			End:           endTime,
			StreamKey:     streamKey,
			PlaylistUrl:   playlist,
			LiveNow:       false,
			Recording:     req.Vodup,
			Premiere:      req.Premiere,
		}

		// add Series Identifier
		if len(req.DateSeries) > 1 {
			lecture.SeriesIdentifier = seriesIdentifier
		}

		// add file if premiere
		if req.Premiere || req.Vodup {
			lecture.Files = []model.File{{Path: fmt.Sprintf("%s/%s", premiereFolder, premiereFileName)}}
		}
		tumLiveContext.Course.Streams = append(tumLiveContext.Course.Streams, lecture)
	}

	err = dao.UpdateCourse(context.Background(), *tumLiveContext.Course)
	if err != nil {
		log.WithError(err).Warn("Can't update course")
	}
}

type createLectureRequest struct {
	Title         string      `json:"title"`
	LectureHallId string      `json:"lectureHallId"`
	Start         time.Time   `json:"start"`
	Duration      int         `json:"duration"`
	Premiere      bool        `json:"premiere"`
	Vodup         bool        `json:"vodup"`
	DateSeries    []time.Time `json:"dateSeries"`
}

func createCourse(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	jsonData, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	var req createCourseRequest
	err = json.Unmarshal(jsonData, &req)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	match, err := regexp.MatchString("(enrolled|public|loggedin|hidden)", req.Access)
	if err != nil || !match {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	//verify teaching term input, should either be Sommersemester 2020 or Wintersemester 2020/21
	match, err = regexp.MatchString("(Sommersemester [0-9]{4}|Wintersemester [0-9]{4}/[0-9]{2})$", req.TeachingTerm)
	if err != nil || !match {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "Semester is not in the correct format"})
		return
	}
	reYear := regexp.MustCompile("[0-9]{4}")
	year, err := strconv.Atoi(reYear.FindStringSubmatch(req.TeachingTerm)[0])
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "Semester is not in the correct format"})
		return
	}
	var semester string
	if strings.Contains(req.TeachingTerm, "Wintersemester") {
		semester = "W"
	} else {
		semester = "S"
	}
	_, err = dao.GetCourseBySlugYearAndTerm(c, req.Slug, semester, year)
	if err == nil {
		c.AbortWithStatusJSON(http.StatusConflict, gin.H{"message": "Course with slug already exists"})
		return
	}

	course := model.Course{
		UserID:              tumLiveContext.User.ID,
		Name:                req.Name,
		Slug:                req.Slug,
		Year:                year,
		TeachingTerm:        semester,
		TUMOnlineIdentifier: req.CourseID,
		VODEnabled:          req.EnVOD,
		DownloadsEnabled:    req.EnDL,
		ChatEnabled:         req.EnChat,
		Visibility:          req.Access,
		Streams:             []model.Stream{},
	}
	if tumLiveContext.User.Role != model.AdminType {
		course.Admins = []model.User{*tumLiveContext.User}
	}
	err = dao.CreateCourse(context.Background(), course, true)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, "Couldn't save course. Please reach out to us.")
		return
	}
	courseWithID, err := dao.GetCourseBySlugYearAndTerm(context.Background(), req.Slug, semester, year)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, "Could not get course for slug and term. Please reach out to us.")
	}
	// refresh enrollments and lectures
	courses := make([]model.Course, 1)
	courses[0] = courseWithID
	go tum.GetEventsForCourses(courses)
	go tum.FindStudentsForCourses(courses)
	go tum.FetchCourses()
}

func deleteCourse(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	tumLiveContext := foundContext.(tools.TUMLiveContext)

	log.WithFields(log.Fields{
		"user":   tumLiveContext.User.ID,
		"course": tumLiveContext.Course.ID,
	}).Info("Delete Course Called")

	dao.DeleteCourse(*tumLiveContext.Course)
	dao.Cache.Clear()
}

type createCourseRequest struct {
	Access       string //enrolled, public, hidden or loggedin
	CourseID     string
	EnChat       bool
	EnDL         bool
	EnVOD        bool
	Name         string
	Slug         string
	TeachingTerm string
}

func courseInfo(c *gin.Context) {
	jsonData, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	var req getCourseRequest
	err = json.Unmarshal(jsonData, &req)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	var courseInfo tum.CourseInfo
	for _, token := range tools.Cfg.Campus.Tokens {
		courseInfo, err = tum.GetCourseInformation(req.CourseID, token)
		if err == nil {
			break
		}
	}
	if err != nil { // course not found
		log.WithError(err).Warn("Error getting course information")
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	c.JSON(200, courseInfo)
}

type getCourseRequest struct {
	CourseID string `json:"courseID"`
}

type deleteLecturesRequest struct {
	StreamIDs []string `json:"streamIDs"`
}
