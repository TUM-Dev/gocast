package api

import (
	"net/http"

	"TUM-Live-Backend/dao"
	"TUM-Live-Backend/model"

	"github.com/gin-gonic/gin"
	"github.com/guregu/null"
	"github.com/julienschmidt/httprouter"
)

var (
	_ = null.Bool{}
)

func configCourseOwnersRouter(router *httprouter.Router) {
	router.GET("/courseowners", GetAllCourseOwners)
	router.POST("/courseowners", AddCourseOwners)
	router.GET("/courseowners/:argID", GetCourseOwners)
	router.PUT("/courseowners/:argID", UpdateCourseOwners)
	router.DELETE("/courseowners/:argID", DeleteCourseOwners)
}

func configGinCourseOwnersRouter(router gin.IRoutes) {
	router.GET("/courseowners", ConverHttprouterToGin(GetAllCourseOwners))
	router.POST("/courseowners", ConverHttprouterToGin(AddCourseOwners))
	router.GET("/courseowners/:argID", ConverHttprouterToGin(GetCourseOwners))
	router.PUT("/courseowners/:argID", ConverHttprouterToGin(UpdateCourseOwners))
	router.DELETE("/courseowners/:argID", ConverHttprouterToGin(DeleteCourseOwners))
}

// GetAllCourseOwners is a function to get a slice of record(s) from course_owners table in the rbglive database
// @Summary Get list of CourseOwners
// @Tags CourseOwners
// @Description GetAllCourseOwners is a handler to get a slice of record(s) from course_owners table in the rbglive database
// @Accept  json
// @Produce  json
// @Param   page     query    int     false        "page requested (defaults to 0)"
// @Param   pagesize query    int     false        "number of records in a page  (defaults to 20)"
// @Param   order    query    string  false        "db sort order column"
// @Success 200 {object} api.PagedResults{data=[]model.CourseOwners}
// @Failure 400 {object} api.HTTPError
// @Failure 404 {object} api.HTTPError
// @Router /courseowners [get]
// http "http://localhost:8080/courseowners?page=0&pagesize=20" X-Api-User:user123
func GetAllCourseOwners(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := initializeContext(r)
	page, err := readInt(r, "page", 0)
	if err != nil || page < 0 {
		returnError(ctx, w, r, dao.ErrBadParams)
		return
	}

	pagesize, err := readInt(r, "pagesize", 20)
	if err != nil || pagesize <= 0 {
		returnError(ctx, w, r, dao.ErrBadParams)
		return
	}

	order := r.FormValue("order")

	if err := ValidateRequest(ctx, r, "course_owners", model.RetrieveMany); err != nil {
		returnError(ctx, w, r, err)
		return
	}

	records, totalRows, err := dao.GetAllCourseOwners(ctx, page, pagesize, order)
	if err != nil {
		returnError(ctx, w, r, err)
		return
	}

	result := &PagedResults{Page: page, PageSize: pagesize, Data: records, TotalRecords: totalRows}
	writeJSON(ctx, w, result)
}

// GetCourseOwners is a function to get a single record from the course_owners table in the rbglive database
// @Summary Get record from table CourseOwners by  argID
// @Tags CourseOwners
// @ID argID
// @Description GetCourseOwners is a function to get a single record from the course_owners table in the rbglive database
// @Accept  json
// @Produce  json
// @Param  argID path int true "id"
// @Success 200 {object} model.CourseOwners
// @Failure 400 {object} api.HTTPError
// @Failure 404 {object} api.HTTPError "ErrNotFound, db record for id not found - returns NotFound HTTP 404 not found error"
// @Router /courseowners/{argID} [get]
// http "http://localhost:8080/courseowners/1" X-Api-User:user123
func GetCourseOwners(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := initializeContext(r)

	argID, err := parseInt32(ps, "argID")
	if err != nil {
		returnError(ctx, w, r, err)
		return
	}

	if err := ValidateRequest(ctx, r, "course_owners", model.RetrieveOne); err != nil {
		returnError(ctx, w, r, err)
		return
	}

	record, err := dao.GetCourseOwners(ctx, argID)
	if err != nil {
		returnError(ctx, w, r, err)
		return
	}

	writeJSON(ctx, w, record)
}

// AddCourseOwners add to add a single record to course_owners table in the rbglive database
// @Summary Add an record to course_owners table
// @Description add to add a single record to course_owners table in the rbglive database
// @Tags CourseOwners
// @Accept  json
// @Produce  json
// @Param CourseOwners body model.CourseOwners true "Add CourseOwners"
// @Success 200 {object} model.CourseOwners
// @Failure 400 {object} api.HTTPError
// @Failure 404 {object} api.HTTPError
// @Router /courseowners [post]
// echo '{"Id": 68,"Userid": 1,"Courseid": 55}' | http POST "http://localhost:8080/courseowners" X-Api-User:user123
func AddCourseOwners(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := initializeContext(r)
	courseowners := &model.CourseOwners{}

	if err := readJSON(r, courseowners); err != nil {
		returnError(ctx, w, r, dao.ErrBadParams)
		return
	}

	if err := courseowners.BeforeSave(); err != nil {
		returnError(ctx, w, r, dao.ErrBadParams)
	}

	courseowners.Prepare()

	if err := courseowners.Validate(model.Create); err != nil {
		returnError(ctx, w, r, dao.ErrBadParams)
		return
	}

	if err := ValidateRequest(ctx, r, "course_owners", model.Create); err != nil {
		returnError(ctx, w, r, err)
		return
	}

	var err error
	courseowners, _, err = dao.AddCourseOwners(ctx, courseowners)
	if err != nil {
		returnError(ctx, w, r, err)
		return
	}

	writeJSON(ctx, w, courseowners)
}

// UpdateCourseOwners Update a single record from course_owners table in the rbglive database
// @Summary Update an record in table course_owners
// @Description Update a single record from course_owners table in the rbglive database
// @Tags CourseOwners
// @Accept  json
// @Produce  json
// @Param  argID path int true "id"
// @Param  CourseOwners body model.CourseOwners true "Update CourseOwners record"
// @Success 200 {object} model.CourseOwners
// @Failure 400 {object} api.HTTPError
// @Failure 404 {object} api.HTTPError
// @Router /courseowners/{argID} [put]
// echo '{"Id": 68,"Userid": 1,"Courseid": 55}' | http PUT "http://localhost:8080/courseowners/1"  X-Api-User:user123
func UpdateCourseOwners(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := initializeContext(r)

	argID, err := parseInt32(ps, "argID")
	if err != nil {
		returnError(ctx, w, r, err)
		return
	}

	courseowners := &model.CourseOwners{}
	if err := readJSON(r, courseowners); err != nil {
		returnError(ctx, w, r, dao.ErrBadParams)
		return
	}

	if err := courseowners.BeforeSave(); err != nil {
		returnError(ctx, w, r, dao.ErrBadParams)
	}

	courseowners.Prepare()

	if err := courseowners.Validate(model.Update); err != nil {
		returnError(ctx, w, r, dao.ErrBadParams)
		return
	}

	if err := ValidateRequest(ctx, r, "course_owners", model.Update); err != nil {
		returnError(ctx, w, r, err)
		return
	}

	courseowners, _, err = dao.UpdateCourseOwners(ctx,
		argID,
		courseowners)
	if err != nil {
		returnError(ctx, w, r, err)
		return
	}

	writeJSON(ctx, w, courseowners)
}

// DeleteCourseOwners Delete a single record from course_owners table in the rbglive database
// @Summary Delete a record from course_owners
// @Description Delete a single record from course_owners table in the rbglive database
// @Tags CourseOwners
// @Accept  json
// @Produce  json
// @Param  argID path int true "id"
// @Success 204 {object} model.CourseOwners
// @Failure 400 {object} api.HTTPError
// @Failure 500 {object} api.HTTPError
// @Router /courseowners/{argID} [delete]
// http DELETE "http://localhost:8080/courseowners/1" X-Api-User:user123
func DeleteCourseOwners(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := initializeContext(r)

	argID, err := parseInt32(ps, "argID")
	if err != nil {
		returnError(ctx, w, r, err)
		return
	}

	if err := ValidateRequest(ctx, r, "course_owners", model.Delete); err != nil {
		returnError(ctx, w, r, err)
		return
	}

	rowsAffected, err := dao.DeleteCourseOwners(ctx, argID)
	if err != nil {
		returnError(ctx, w, r, err)
		return
	}

	writeRowsAffected(w, rowsAffected)
}
