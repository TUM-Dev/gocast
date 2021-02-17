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

func configCoursesRouter(router *httprouter.Router) {
	router.GET("/courses", GetAllCourses)
	router.POST("/courses", AddCourses)
	router.GET("/courses/:argID", GetCourses)
	router.PUT("/courses/:argID", UpdateCourses)
	router.DELETE("/courses/:argID", DeleteCourses)
}

func configGinCoursesRouter(router gin.IRoutes) {
	router.GET("/courses", ConverHttprouterToGin(GetAllCourses))
	router.POST("/courses", ConverHttprouterToGin(AddCourses))
	router.GET("/courses/:argID", ConverHttprouterToGin(GetCourses))
	router.PUT("/courses/:argID", ConverHttprouterToGin(UpdateCourses))
	router.DELETE("/courses/:argID", ConverHttprouterToGin(DeleteCourses))
}

// GetAllCourses is a function to get a slice of record(s) from courses table in the rbglive database
// @Summary Get list of Courses
// @Tags Courses
// @Description GetAllCourses is a handler to get a slice of record(s) from courses table in the rbglive database
// @Accept  json
// @Produce  json
// @Param   page     query    int     false        "page requested (defaults to 0)"
// @Param   pagesize query    int     false        "number of records in a page  (defaults to 20)"
// @Param   order    query    string  false        "db sort order column"
// @Success 200 {object} api.PagedResults{data=[]model.Courses}
// @Failure 400 {object} api.HTTPError
// @Failure 404 {object} api.HTTPError
// @Router /courses [get]
// http "http://localhost:8080/courses?page=0&pagesize=20" X-Api-User:user123
func GetAllCourses(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
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

	if err := ValidateRequest(ctx, r, "courses", model.RetrieveMany); err != nil {
		returnError(ctx, w, r, err)
		return
	}

	records, totalRows, err := dao.GetAllCourses(ctx, page, pagesize, order)
	if err != nil {
		returnError(ctx, w, r, err)
		return
	}

	result := &PagedResults{Page: page, PageSize: pagesize, Data: records, TotalRecords: totalRows}
	writeJSON(ctx, w, result)
}

// GetCourses is a function to get a single record from the courses table in the rbglive database
// @Summary Get record from table Courses by  argID
// @Tags Courses
// @ID argID
// @Description GetCourses is a function to get a single record from the courses table in the rbglive database
// @Accept  json
// @Produce  json
// @Param  argID path int true "id"
// @Success 200 {object} model.Courses
// @Failure 400 {object} api.HTTPError
// @Failure 404 {object} api.HTTPError "ErrNotFound, db record for id not found - returns NotFound HTTP 404 not found error"
// @Router /courses/{argID} [get]
// http "http://localhost:8080/courses/1" X-Api-User:user123
func GetCourses(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := initializeContext(r)

	argID, err := parseInt32(ps, "argID")
	if err != nil {
		returnError(ctx, w, r, err)
		return
	}

	if err := ValidateRequest(ctx, r, "courses", model.RetrieveOne); err != nil {
		returnError(ctx, w, r, err)
		return
	}

	record, err := dao.GetCourses(ctx, argID)
	if err != nil {
		returnError(ctx, w, r, err)
		return
	}

	writeJSON(ctx, w, record)
}

// AddCourses add to add a single record to courses table in the rbglive database
// @Summary Add an record to courses table
// @Description add to add a single record to courses table in the rbglive database
// @Tags Courses
// @Accept  json
// @Produce  json
// @Param Courses body model.Courses true "Add Courses"
// @Success 200 {object} model.Courses
// @Failure 400 {object} api.HTTPError
// @Failure 404 {object} api.HTTPError
// @Router /courses [post]
// echo '{"Id": 44,"Name": "eLtHjUNZxaXhuedVENIQYdaxw","Start": "2165-08-09T01:05:44.329163825+01:00","End": "2290-11-16T04:16:47.5391697+01:00","Semester": "jXUyKeprFSkvCZqCeDAERBTbY"}' | http POST "http://localhost:8080/courses" X-Api-User:user123
func AddCourses(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := initializeContext(r)
	courses := &model.Courses{}

	if err := readJSON(r, courses); err != nil {
		returnError(ctx, w, r, dao.ErrBadParams)
		return
	}

	if err := courses.BeforeSave(); err != nil {
		returnError(ctx, w, r, dao.ErrBadParams)
	}

	courses.Prepare()

	if err := courses.Validate(model.Create); err != nil {
		returnError(ctx, w, r, dao.ErrBadParams)
		return
	}

	if err := ValidateRequest(ctx, r, "courses", model.Create); err != nil {
		returnError(ctx, w, r, err)
		return
	}

	var err error
	courses, _, err = dao.AddCourses(ctx, courses)
	if err != nil {
		returnError(ctx, w, r, err)
		return
	}

	writeJSON(ctx, w, courses)
}

// UpdateCourses Update a single record from courses table in the rbglive database
// @Summary Update an record in table courses
// @Description Update a single record from courses table in the rbglive database
// @Tags Courses
// @Accept  json
// @Produce  json
// @Param  argID path int true "id"
// @Param  Courses body model.Courses true "Update Courses record"
// @Success 200 {object} model.Courses
// @Failure 400 {object} api.HTTPError
// @Failure 404 {object} api.HTTPError
// @Router /courses/{argID} [put]
// echo '{"Id": 44,"Name": "eLtHjUNZxaXhuedVENIQYdaxw","Start": "2165-08-09T01:05:44.329163825+01:00","End": "2290-11-16T04:16:47.5391697+01:00","Semester": "jXUyKeprFSkvCZqCeDAERBTbY"}' | http PUT "http://localhost:8080/courses/1"  X-Api-User:user123
func UpdateCourses(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := initializeContext(r)

	argID, err := parseInt32(ps, "argID")
	if err != nil {
		returnError(ctx, w, r, err)
		return
	}

	courses := &model.Courses{}
	if err := readJSON(r, courses); err != nil {
		returnError(ctx, w, r, dao.ErrBadParams)
		return
	}

	if err := courses.BeforeSave(); err != nil {
		returnError(ctx, w, r, dao.ErrBadParams)
	}

	courses.Prepare()

	if err := courses.Validate(model.Update); err != nil {
		returnError(ctx, w, r, dao.ErrBadParams)
		return
	}

	if err := ValidateRequest(ctx, r, "courses", model.Update); err != nil {
		returnError(ctx, w, r, err)
		return
	}

	courses, _, err = dao.UpdateCourses(ctx,
		argID,
		courses)
	if err != nil {
		returnError(ctx, w, r, err)
		return
	}

	writeJSON(ctx, w, courses)
}

// DeleteCourses Delete a single record from courses table in the rbglive database
// @Summary Delete a record from courses
// @Description Delete a single record from courses table in the rbglive database
// @Tags Courses
// @Accept  json
// @Produce  json
// @Param  argID path int true "id"
// @Success 204 {object} model.Courses
// @Failure 400 {object} api.HTTPError
// @Failure 500 {object} api.HTTPError
// @Router /courses/{argID} [delete]
// http DELETE "http://localhost:8080/courses/1" X-Api-User:user123
func DeleteCourses(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := initializeContext(r)

	argID, err := parseInt32(ps, "argID")
	if err != nil {
		returnError(ctx, w, r, err)
		return
	}

	if err := ValidateRequest(ctx, r, "courses", model.Delete); err != nil {
		returnError(ctx, w, r, err)
		return
	}

	rowsAffected, err := dao.DeleteCourses(ctx, argID)
	if err != nil {
		returnError(ctx, w, r, err)
		return
	}

	writeRowsAffected(w, rowsAffected)
}
