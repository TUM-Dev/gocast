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

func configStreamsRouter(router *httprouter.Router) {
	router.GET("/streams", GetAllStreams)
	router.POST("/streams", AddStreams)
	router.GET("/streams/:argID", GetStreams)
	router.PUT("/streams/:argID", UpdateStreams)
	router.DELETE("/streams/:argID", DeleteStreams)
}

func configGinStreamsRouter(router gin.IRoutes) {
	router.GET("/streams", ConverHttprouterToGin(GetAllStreams))
	router.POST("/streams", ConverHttprouterToGin(AddStreams))
	router.GET("/streams/:argID", ConverHttprouterToGin(GetStreams))
	router.PUT("/streams/:argID", ConverHttprouterToGin(UpdateStreams))
	router.DELETE("/streams/:argID", ConverHttprouterToGin(DeleteStreams))
}

// GetAllStreams is a function to get a slice of record(s) from streams table in the rbglive database
// @Summary Get list of Streams
// @Tags Streams
// @Description GetAllStreams is a handler to get a slice of record(s) from streams table in the rbglive database
// @Accept  json
// @Produce  json
// @Param   page     query    int     false        "page requested (defaults to 0)"
// @Param   pagesize query    int     false        "number of records in a page  (defaults to 20)"
// @Param   order    query    string  false        "db sort order column"
// @Success 200 {object} api.PagedResults{data=[]model.Streams}
// @Failure 400 {object} api.HTTPError
// @Failure 404 {object} api.HTTPError
// @Router /streams [get]
// http "http://localhost:8080/streams?page=0&pagesize=20" X-Api-User:user123
func GetAllStreams(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
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

	if err := ValidateRequest(ctx, r, "streams", model.RetrieveMany); err != nil {
		returnError(ctx, w, r, err)
		return
	}

	records, totalRows, err := dao.GetAllStreams(ctx, page, pagesize, order)
	if err != nil {
		returnError(ctx, w, r, err)
		return
	}

	result := &PagedResults{Page: page, PageSize: pagesize, Data: records, TotalRecords: totalRows}
	writeJSON(ctx, w, result)
}

// GetStreams is a function to get a single record from the streams table in the rbglive database
// @Summary Get record from table Streams by  argID
// @Tags Streams
// @ID argID
// @Description GetStreams is a function to get a single record from the streams table in the rbglive database
// @Accept  json
// @Produce  json
// @Param  argID path int true "id"
// @Success 200 {object} model.Streams
// @Failure 400 {object} api.HTTPError
// @Failure 404 {object} api.HTTPError "ErrNotFound, db record for id not found - returns NotFound HTTP 404 not found error"
// @Router /streams/{argID} [get]
// http "http://localhost:8080/streams/1" X-Api-User:user123
func GetStreams(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := initializeContext(r)

	argID, err := parseInt32(ps, "argID")
	if err != nil {
		returnError(ctx, w, r, err)
		return
	}

	if err := ValidateRequest(ctx, r, "streams", model.RetrieveOne); err != nil {
		returnError(ctx, w, r, err)
		return
	}

	record, err := dao.GetStreams(ctx, argID)
	if err != nil {
		returnError(ctx, w, r, err)
		return
	}

	writeJSON(ctx, w, record)
}

// AddStreams add to add a single record to streams table in the rbglive database
// @Summary Add an record to streams table
// @Description add to add a single record to streams table in the rbglive database
// @Tags Streams
// @Accept  json
// @Produce  json
// @Param Streams body model.Streams true "Add Streams"
// @Success 200 {object} model.Streams
// @Failure 400 {object} api.HTTPError
// @Failure 404 {object} api.HTTPError
// @Router /streams [post]
// echo '{"VodEnabled": false,"Id": 93,"Start": "2261-09-17T03:50:49.948545483+01:00","End": "2194-05-27T15:35:51.008796589+01:00","Streamkey": "GckbWqPJopOtNvaKrAAHkiESA","CourseId": 48}' | http POST "http://localhost:8080/streams" X-Api-User:user123
func AddStreams(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := initializeContext(r)
	streams := &model.Streams{}

	if err := readJSON(r, streams); err != nil {
		returnError(ctx, w, r, dao.ErrBadParams)
		return
	}

	if err := streams.BeforeSave(); err != nil {
		returnError(ctx, w, r, dao.ErrBadParams)
	}

	streams.Prepare()

	if err := streams.Validate(model.Create); err != nil {
		returnError(ctx, w, r, dao.ErrBadParams)
		return
	}

	if err := ValidateRequest(ctx, r, "streams", model.Create); err != nil {
		returnError(ctx, w, r, err)
		return
	}

	var err error
	streams, _, err = dao.AddStreams(ctx, streams)
	if err != nil {
		returnError(ctx, w, r, err)
		return
	}

	writeJSON(ctx, w, streams)
}

// UpdateStreams Update a single record from streams table in the rbglive database
// @Summary Update an record in table streams
// @Description Update a single record from streams table in the rbglive database
// @Tags Streams
// @Accept  json
// @Produce  json
// @Param  argID path int true "id"
// @Param  Streams body model.Streams true "Update Streams record"
// @Success 200 {object} model.Streams
// @Failure 400 {object} api.HTTPError
// @Failure 404 {object} api.HTTPError
// @Router /streams/{argID} [put]
// echo '{"VodEnabled": false,"Id": 93,"Start": "2261-09-17T03:50:49.948545483+01:00","End": "2194-05-27T15:35:51.008796589+01:00","Streamkey": "GckbWqPJopOtNvaKrAAHkiESA","CourseId": 48}' | http PUT "http://localhost:8080/streams/1"  X-Api-User:user123
func UpdateStreams(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := initializeContext(r)

	argID, err := parseInt32(ps, "argID")
	if err != nil {
		returnError(ctx, w, r, err)
		return
	}

	streams := &model.Streams{}
	if err := readJSON(r, streams); err != nil {
		returnError(ctx, w, r, dao.ErrBadParams)
		return
	}

	if err := streams.BeforeSave(); err != nil {
		returnError(ctx, w, r, dao.ErrBadParams)
	}

	streams.Prepare()

	if err := streams.Validate(model.Update); err != nil {
		returnError(ctx, w, r, dao.ErrBadParams)
		return
	}

	if err := ValidateRequest(ctx, r, "streams", model.Update); err != nil {
		returnError(ctx, w, r, err)
		return
	}

	streams, _, err = dao.UpdateStreams(ctx,
		argID,
		streams)
	if err != nil {
		returnError(ctx, w, r, err)
		return
	}

	writeJSON(ctx, w, streams)
}

// DeleteStreams Delete a single record from streams table in the rbglive database
// @Summary Delete a record from streams
// @Description Delete a single record from streams table in the rbglive database
// @Tags Streams
// @Accept  json
// @Produce  json
// @Param  argID path int true "id"
// @Success 204 {object} model.Streams
// @Failure 400 {object} api.HTTPError
// @Failure 500 {object} api.HTTPError
// @Router /streams/{argID} [delete]
// http DELETE "http://localhost:8080/streams/1" X-Api-User:user123
func DeleteStreams(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := initializeContext(r)

	argID, err := parseInt32(ps, "argID")
	if err != nil {
		returnError(ctx, w, r, err)
		return
	}

	if err := ValidateRequest(ctx, r, "streams", model.Delete); err != nil {
		returnError(ctx, w, r, err)
		return
	}

	rowsAffected, err := dao.DeleteStreams(ctx, argID)
	if err != nil {
		returnError(ctx, w, r, err)
		return
	}

	writeRowsAffected(w, rowsAffected)
}
