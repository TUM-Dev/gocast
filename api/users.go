package api

import (
	"TUM-Live-Backend/dao"
	"TUM-Live-Backend/model"
	"context"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/julienschmidt/httprouter"
	uuid "github.com/satori/go.uuid"
	"log"
	"net/http"
	"time"
)

func configGinUsersRouter(router gin.IRoutes) {
	router.POST("/api/createUser", ConvertHttprouterToGin(CreateUser))
	router.POST("/api/login", ConvertHttprouterToGin(Login))
}

type loginRequest struct {
	Email    string
	Password string
}

func Login(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	var requestData loginRequest
	err := json.NewDecoder(request.Body).Decode(&requestData)
	if err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		return
	}
	user, err := dao.GetUserByEmail(context.Background(), requestData.Email)
	if err != nil {
		writer.WriteHeader(http.StatusUnauthorized)
		return
	}
	pwCorrect, err := user.ComparePasswordAndHash(requestData.Password)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		log.Printf("error validating password: %v\n", err)
	}
	if pwCorrect {
		var cookie http.Cookie
		cookie.Name = "SID"
		cookie.Value = uuid.NewV4().String()
		cookie.Expires = time.Now().AddDate(0, 1, 0)
		cookie.Path = "/"
		var session model.Session
		session.User = user
		session.SessionID = cookie.Value
		session.UserID = user.ID
		err = dao.CreateSession(context.Background(), session)
		if err != nil {
			log.Printf("couldn't create session: %v\n", err)
			writer.WriteHeader(http.StatusInternalServerError)
			return
		} else {
			writer.WriteHeader(http.StatusOK)
			http.SetCookie(writer, &cookie)
			return
		}
	}
	writer.WriteHeader(http.StatusInternalServerError)
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
	var user = model.User{}
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
