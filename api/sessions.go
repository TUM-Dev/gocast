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

func configSessionsRouter(router *httprouter.Router) {
	router.GET("/sessions", GetAllSessions)
	router.POST("/sessions", AddSessions)
	router.GET("/sessions/:argID", GetSessions)
	router.PUT("/sessions/:argID", UpdateSessions)
	router.DELETE("/sessions/:argID", DeleteSessions)
}

func configGinSessionsRouter(router gin.IRoutes) {
	router.GET("/sessions", ConverHttprouterToGin(GetAllSessions))
	router.POST("/sessions", ConverHttprouterToGin(AddSessions))
	router.GET("/sessions/:argID", ConverHttprouterToGin(GetSessions))
	router.PUT("/sessions/:argID", ConverHttprouterToGin(UpdateSessions))
	router.DELETE("/sessions/:argID", ConverHttprouterToGin(DeleteSessions))
}

// GetAllSessions is a function to get a slice of record(s) from sessions table in the rbglive database
// @Summary Get list of Sessions
// @Tags Sessions
// @Description GetAllSessions is a handler to get a slice of record(s) from sessions table in the rbglive database
// @Accept  json
// @Produce  json
// @Param   page     query    int     false        "page requested (defaults to 0)"
// @Param   pagesize query    int     false        "number of records in a page  (defaults to 20)"
// @Param   order    query    string  false        "db sort order column"
// @Success 200 {object} api.PagedResults{data=[]model.Sessions}
// @Failure 400 {object} api.HTTPError
// @Failure 404 {object} api.HTTPError
// @Router /sessions [get]
// http "http://localhost:8080/sessions?page=0&pagesize=20" X-Api-User:user123
func GetAllSessions(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
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

	if err := ValidateRequest(ctx, r, "sessions", model.RetrieveMany); err != nil {
		returnError(ctx, w, r, err)
		return
	}

	records, totalRows, err := dao.GetAllSessions(ctx, page, pagesize, order)
	if err != nil {
		returnError(ctx, w, r, err)
		return
	}

	result := &PagedResults{Page: page, PageSize: pagesize, Data: records, TotalRecords: totalRows}
	writeJSON(ctx, w, result)
}

// GetSessions is a function to get a single record from the sessions table in the rbglive database
// @Summary Get record from table Sessions by  argID
// @Tags Sessions
// @ID argID
// @Description GetSessions is a function to get a single record from the sessions table in the rbglive database
// @Accept  json
// @Produce  json
// @Param  argID path int true "id"
// @Success 200 {object} model.Sessions
// @Failure 400 {object} api.HTTPError
// @Failure 404 {object} api.HTTPError "ErrNotFound, db record for id not found - returns NotFound HTTP 404 not found error"
// @Router /sessions/{argID} [get]
// http "http://localhost:8080/sessions/1" X-Api-User:user123
func GetSessions(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := initializeContext(r)

	argID, err := parseInt32(ps, "argID")
	if err != nil {
		returnError(ctx, w, r, err)
		return
	}

	if err := ValidateRequest(ctx, r, "sessions", model.RetrieveOne); err != nil {
		returnError(ctx, w, r, err)
		return
	}

	record, err := dao.GetSessions(ctx, argID)
	if err != nil {
		returnError(ctx, w, r, err)
		return
	}

	writeJSON(ctx, w, record)
}

// AddSessions add to add a single record to sessions table in the rbglive database
// @Summary Add an record to sessions table
// @Description add to add a single record to sessions table in the rbglive database
// @Tags Sessions
// @Accept  json
// @Produce  json
// @Param Sessions body model.Sessions true "Add Sessions"
// @Success 200 {object} model.Sessions
// @Failure 400 {object} api.HTTPError
// @Failure 404 {object} api.HTTPError
// @Router /sessions [post]
// echo '{"UserId": 41,"Id": 50,"Created": "2215-03-21T15:22:28.97951483+01:00","SessionId": "DYkxqsubcmcJIGhIvfJKdCbTw"}' | http POST "http://localhost:8080/sessions" X-Api-User:user123
func AddSessions(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := initializeContext(r)
	sessions := &model.Sessions{}

	if err := readJSON(r, sessions); err != nil {
		returnError(ctx, w, r, dao.ErrBadParams)
		return
	}

	if err := sessions.BeforeSave(); err != nil {
		returnError(ctx, w, r, dao.ErrBadParams)
	}

	sessions.Prepare()

	if err := sessions.Validate(model.Create); err != nil {
		returnError(ctx, w, r, dao.ErrBadParams)
		return
	}

	if err := ValidateRequest(ctx, r, "sessions", model.Create); err != nil {
		returnError(ctx, w, r, err)
		return
	}

	var err error
	sessions, _, err = dao.AddSessions(ctx, sessions)
	if err != nil {
		returnError(ctx, w, r, err)
		return
	}

	writeJSON(ctx, w, sessions)
}

// UpdateSessions Update a single record from sessions table in the rbglive database
// @Summary Update an record in table sessions
// @Description Update a single record from sessions table in the rbglive database
// @Tags Sessions
// @Accept  json
// @Produce  json
// @Param  argID path int true "id"
// @Param  Sessions body model.Sessions true "Update Sessions record"
// @Success 200 {object} model.Sessions
// @Failure 400 {object} api.HTTPError
// @Failure 404 {object} api.HTTPError
// @Router /sessions/{argID} [put]
// echo '{"UserId": 41,"Id": 50,"Created": "2215-03-21T15:22:28.97951483+01:00","SessionId": "DYkxqsubcmcJIGhIvfJKdCbTw"}' | http PUT "http://localhost:8080/sessions/1"  X-Api-User:user123
func UpdateSessions(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := initializeContext(r)

	argID, err := parseInt32(ps, "argID")
	if err != nil {
		returnError(ctx, w, r, err)
		return
	}

	sessions := &model.Sessions{}
	if err := readJSON(r, sessions); err != nil {
		returnError(ctx, w, r, dao.ErrBadParams)
		return
	}

	if err := sessions.BeforeSave(); err != nil {
		returnError(ctx, w, r, dao.ErrBadParams)
	}

	sessions.Prepare()

	if err := sessions.Validate(model.Update); err != nil {
		returnError(ctx, w, r, dao.ErrBadParams)
		return
	}

	if err := ValidateRequest(ctx, r, "sessions", model.Update); err != nil {
		returnError(ctx, w, r, err)
		return
	}

	sessions, _, err = dao.UpdateSessions(ctx,
		argID,
		sessions)
	if err != nil {
		returnError(ctx, w, r, err)
		return
	}

	writeJSON(ctx, w, sessions)
}

// DeleteSessions Delete a single record from sessions table in the rbglive database
// @Summary Delete a record from sessions
// @Description Delete a single record from sessions table in the rbglive database
// @Tags Sessions
// @Accept  json
// @Produce  json
// @Param  argID path int true "id"
// @Success 204 {object} model.Sessions
// @Failure 400 {object} api.HTTPError
// @Failure 500 {object} api.HTTPError
// @Router /sessions/{argID} [delete]
// http DELETE "http://localhost:8080/sessions/1" X-Api-User:user123
func DeleteSessions(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := initializeContext(r)

	argID, err := parseInt32(ps, "argID")
	if err != nil {
		returnError(ctx, w, r, err)
		return
	}

	if err := ValidateRequest(ctx, r, "sessions", model.Delete); err != nil {
		returnError(ctx, w, r, err)
		return
	}

	rowsAffected, err := dao.DeleteSessions(ctx, argID)
	if err != nil {
		returnError(ctx, w, r, err)
		return
	}

	writeRowsAffected(w, rowsAffected)
}
