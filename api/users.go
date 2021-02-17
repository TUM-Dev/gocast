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

func configUsersRouter(router *httprouter.Router) {
	router.GET("/users", GetAllUsers)
	router.POST("/users", AddUser)
	router.GET("/users/:argID", GetUsers)
	router.PUT("/users/:argID", UpdateUsers)
	router.DELETE("/users/:argID", DeleteUser)
}

func configGinUsersRouter(router gin.IRoutes) {
	router.GET("/users", ConverHttprouterToGin(GetAllUsers))
	router.POST("/users", ConverHttprouterToGin(AddUser))
	router.GET("/users/:argID", ConverHttprouterToGin(GetUsers))
	router.PUT("/users/:argID", ConverHttprouterToGin(UpdateUsers))
	router.DELETE("/users/:argID", ConverHttprouterToGin(DeleteUser))
}

// GetAllUsers is a function to get a slice of record(s) from users table in the rbglive database
// @Summary Get list of Users
// @Tags Users
// @Description GetAllUsers is a handler to get a slice of record(s) from users table in the rbglive database
// @Accept  json
// @Produce  json
// @Param   page     query    int     false        "page requested (defaults to 0)"
// @Param   pagesize query    int     false        "number of records in a page  (defaults to 20)"
// @Param   order    query    string  false        "db sort order column"
// @Success 200 {object} api.PagedResults{data=[]model.Users}
// @Failure 400 {object} api.HTTPError
// @Failure 404 {object} api.HTTPError
// @Router /users [get]
// http "http://localhost:8080/users?page=0&pagesize=20" X-Api-User:user123
func GetAllUsers(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
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

	if err := ValidateRequest(ctx, r, "users", model.RetrieveMany); err != nil {
		returnError(ctx, w, r, err)
		return
	}

	records, totalRows, err := dao.GetAllUsers(ctx, page, pagesize, order)
	if err != nil {
		returnError(ctx, w, r, err)
		return
	}

	result := &PagedResults{Page: page, PageSize: pagesize, Data: records, TotalRecords: totalRows}
	writeJSON(ctx, w, result)
}

// GetUsers is a function to get a single record from the users table in the rbglive database
// @Summary Get record from table Users by  argID
// @Tags Users
// @ID argID
// @Description GetUsers is a function to get a single record from the users table in the rbglive database
// @Accept  json
// @Produce  json
// @Param  argID path int true "id"
// @Success 200 {object} model.Users
// @Failure 400 {object} api.HTTPError
// @Failure 404 {object} api.HTTPError "ErrNotFound, db record for id not found - returns NotFound HTTP 404 not found error"
// @Router /users/{argID} [get]
// http "http://localhost:8080/users/1" X-Api-User:user123
func GetUsers(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := initializeContext(r)

	argID, err := parseInt32(ps, "argID")
	if err != nil {
		returnError(ctx, w, r, err)
		return
	}

	if err := ValidateRequest(ctx, r, "users", model.RetrieveOne); err != nil {
		returnError(ctx, w, r, err)
		return
	}

	record, err := dao.GetUsers(ctx, argID)
	if err != nil {
		returnError(ctx, w, r, err)
		return
	}

	writeJSON(ctx, w, record)
}

// AddUser add to add a single record to users table in the rbglive database
// @Summary Add an record to users table
// @Description add to add a single record to users table in the rbglive database
// @Tags Users
// @Accept  json
// @Produce  json
// @Param Users body model.Users true "Add Users"
// @Success 200 {object} model.Users
// @Failure 400 {object} api.HTTPError
// @Failure 404 {object} api.HTTPError
// @Router /users [post]
// echo '{"Id": 63,"Name": "xrRBRHspGiUhkaSmMcsXfhGWa","Email": "mPYaOxkGxwWHnUNrurPNmBimA","Role": "JnFdCPXDVOTHVpagBSnOcQfum","Password": "ltrurBDUVKlsvVxiWkgXScyxt"}' | http POST "http://localhost:8080/users" X-Api-User:user123
func AddUser(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := initializeContext(r)
	users := &model.Users{}

	if err := readJSON(r, users); err != nil {
		returnError(ctx, w, r, dao.ErrBadParams)
		return
	}

	if err := users.BeforeSave(); err != nil {
		returnError(ctx, w, r, dao.ErrBadParams)
	}

	users.Prepare()

	if err := users.Validate(model.Create); err != nil {
		returnError(ctx, w, r, dao.ErrBadParams)
		return
	}

	if err := ValidateRequest(ctx, r, "users", model.Create); err != nil {
		returnError(ctx, w, r, err)
		return
	}

	var err error
	users, _, err = dao.AddUsers(ctx, users)
	if err != nil {
		returnError(ctx, w, r, err)
		return
	}

	writeJSON(ctx, w, users)
}

// UpdateUsers Update a single record from users table in the rbglive database
// @Summary Update an record in table users
// @Description Update a single record from users table in the rbglive database
// @Tags Users
// @Accept  json
// @Produce  json
// @Param  argID path int true "id"
// @Param  Users body model.Users true "Update Users record"
// @Success 200 {object} model.Users
// @Failure 400 {object} api.HTTPError
// @Failure 404 {object} api.HTTPError
// @Router /users/{argID} [put]
// echo '{"Id": 63,"Name": "xrRBRHspGiUhkaSmMcsXfhGWa","Email": "mPYaOxkGxwWHnUNrurPNmBimA","Role": "JnFdCPXDVOTHVpagBSnOcQfum","Password": "ltrurBDUVKlsvVxiWkgXScyxt"}' | http PUT "http://localhost:8080/users/1"  X-Api-User:user123
func UpdateUsers(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := initializeContext(r)

	argID, err := parseInt32(ps, "argID")
	if err != nil {
		returnError(ctx, w, r, err)
		return
	}

	users := &model.Users{}
	if err := readJSON(r, users); err != nil {
		returnError(ctx, w, r, dao.ErrBadParams)
		return
	}

	if err := users.BeforeSave(); err != nil {
		returnError(ctx, w, r, dao.ErrBadParams)
	}

	users.Prepare()

	if err := users.Validate(model.Update); err != nil {
		returnError(ctx, w, r, dao.ErrBadParams)
		return
	}

	if err := ValidateRequest(ctx, r, "users", model.Update); err != nil {
		returnError(ctx, w, r, err)
		return
	}

	users, _, err = dao.UpdateUser(ctx,
		argID,
		users)
	if err != nil {
		returnError(ctx, w, r, err)
		return
	}

	writeJSON(ctx, w, users)
}

// DeleteUser Delete a single record from users table in the rbglive database
// @Summary Delete a record from users
// @Description Delete a single record from users table in the rbglive database
// @Tags Users
// @Accept  json
// @Produce  json
// @Param  argID path int true "id"
// @Success 204 {object} model.Users
// @Failure 400 {object} api.HTTPError
// @Failure 500 {object} api.HTTPError
// @Router /users/{argID} [delete]
// http DELETE "http://localhost:8080/users/1" X-Api-User:user123
func DeleteUser(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := initializeContext(r)

	argID, err := parseInt32(ps, "argID")
	if err != nil {
		returnError(ctx, w, r, err)
		return
	}

	if err := ValidateRequest(ctx, r, "users", model.Delete); err != nil {
		returnError(ctx, w, r, err)
		return
	}

	rowsAffected, err := dao.DeleteUser(ctx, argID)
	if err != nil {
		returnError(ctx, w, r, err)
		return
	}

	writeRowsAffected(w, rowsAffected)
}
