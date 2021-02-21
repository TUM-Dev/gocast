package api

import (
	"TUM-Live-Backend/dao"
	"TUM-Live-Backend/model"
	"context"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/julienschmidt/httprouter"
	"log"
	"net/http"
)

func configGinUsersRouter(router gin.IRoutes) {
	router.POST("/createUser", ConverHttprouterToGin(CreateUser))
}

func CreateUser(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	usersEmpty, err := dao.AreUsersEmpty(context.Background())
	if !usersEmpty {
		log.Printf("create user request rejected")
		w.WriteHeader(http.StatusForbidden)
		return
	}
	if err != nil {
		log.Printf("couldn't query users: %v\n", err)
		w.WriteHeader(500)
		return
	}
	var request createUserRequest
	err = json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Printf("couldn't decode json: %v\n", err)
		return
	}
	var user model.User
	user.Email = request.Email
	user.Name = request.Name
	user.Role = "admin"
	err = user.SetPassword(request.Password)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("couldn't set password: %v\n", err)
		return
	}
	if !user.ValidateFields() {
		w.WriteHeader(http.StatusBadRequest)
		log.Printf("created user does not meet requirements.")
		return
	}
	err = dao.CreateUser(context.Background(), user)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("couldn't create user: %v\n", err)
		return
	}
	writeJSON(context.Background(), w, createUserResponse{Success: true})
}

type createUserRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type createUserResponse struct {
	Success bool `json:"success"`
}
