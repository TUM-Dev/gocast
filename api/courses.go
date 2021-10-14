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
	"io"
	"io/ioutil"
	"mime/multipart"
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
	adminOfCourseGroup.POST("/createLecture", createLecture)
	adminOfCourseGroup.POST("/deleteLecture/:streamID", deleteLecture)
	adminOfCourseGroup.POST("/renameLecture/:streamID", renameLecture)
	adminOfCourseGroup.POST("/updateDescription/:streamID", updateDescription)
	adminOfCourseGroup.POST("/addUnit", addUnit)
	adminOfCourseGroup.POST("/submitCut", submitCut)
	adminOfCourseGroup.POST("/deleteUnit/:unitID", deleteUnit)
	adminOfCourseGroup.GET("/stats", getStats)
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
	if tumLiveContext.User.Role != model.AdminType && tumLiveContext.User.ID != course.UserID {
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
	var req renameLectureRequest
	jsonData, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	if err := json.Unmarshal(jsonData, &req); err != nil {
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
}

func renameLecture(c *gin.Context) {
	var req renameLectureRequest
	jsonData, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	if err := json.Unmarshal(jsonData, &req); err != nil {
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
}

type renameLectureRequest struct {
	Name string
}

func deleteLecture(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	stream, err := dao.GetStreamByID(context.Background(), c.Param("streamID"))
	if err != nil || stream.CourseID != tumLiveContext.Course.ID {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	stream.Model.DeletedAt = gorm.DeletedAt{Time: time.Now()} // todo ?!
	dao.DeleteStream(strconv.Itoa(int(stream.ID)))
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
	// name for folder for premiere file if needed
	premiereFolder := fmt.Sprintf("%s/%d/%s/%s",
		tools.Cfg.MassStorage,
		tumLiveContext.Course.Year,
		tumLiveContext.Course.TeachingTerm,
		tumLiveContext.Course.Slug)
	premiereFileName := fmt.Sprintf("%s_%s.mp4",
		tumLiveContext.Course.Slug,
		req.Start.Format("2006-01-02_15-04"))
	if req.Premiere {
		err := os.MkdirAll(premiereFolder, os.ModePerm)
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			log.WithError(err).Error("Can't create folder for premiere")
			return
		}
		// Copy file to shared storage
		file, err := os.Create(fmt.Sprintf("%s/%s", premiereFolder, premiereFileName))
		if err != nil {
			log.WithError(err).Error("Can't create file for premiere")
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		reqFile, _ := req.File.Open()
		_, err = io.Copy(file, reqFile)
		if err != nil {
			log.WithError(err).Error("Can't write file for premiere")
			c.AbortWithStatus(http.StatusInternalServerError)
		}
		_ = file.Close()
	}
	streamKey := uuid.NewV4().String()
	streamKey = strings.ReplaceAll(streamKey, "-", "")
	lecture := model.Stream{
		Name:        req.Title,
		CourseID:    tumLiveContext.Course.ID,
		Start:       req.Start,
		End:         req.End,
		StreamKey:   streamKey,
		PlaylistUrl: "",
		LiveNow:     false,
		Premiere:    req.Premiere,
	}
	// add file if premiere
	if req.Premiere {
		lecture.Files = []model.File{{Path: fmt.Sprintf("%s/%s", premiereFolder, premiereFileName)}}
	}
	tumLiveContext.Course.Streams = append(tumLiveContext.Course.Streams, lecture)
	err := dao.UpdateCourse(context.Background(), *tumLiveContext.Course)
	if err != nil {
		log.WithError(err).Warn("Can't update course")
	}
}

type createLectureRequest struct {
	Title    string                `form:"title"`
	Start    time.Time             `form:"start"`
	End      time.Time             `form:"end"`
	Premiere bool                  `form:"premiere"`
	File     *multipart.FileHeader `form:"file"`
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
	err = dao.CreateCourse(context.Background(), course, true)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, "Couldn't save course. Please reach out to us.")
		return
	}
	courseWithID, err := dao.GetCourseBySlugYearAndTerm(context.Background(), req.Slug, semester, fmt.Sprintf("%v", year))
	// refresh enrollments and lectures
	courses := make([]model.Course, 1)
	courses[0] = courseWithID
	go tum.GetEventsForCourses(courses)
	go tum.FindStudentsForCourses(courses)
	go tum.FetchCourses()
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
	for _, token := range tools.Cfg.CampusToken {
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
