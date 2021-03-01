package api

import (
	"TUM-Live/dao"
	"TUM-Live/model"
	"TUM-Live/tools"
	"context"
	"encoding/json"
	"errors"
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

func Login(writer http.ResponseWriter, request *http.Request, _ httprouter.Params) {
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
	writer.WriteHeader(http.StatusUnauthorized)
}

// todo: refactor
func CreateUser(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	usersEmpty, err := dao.AreUsersEmpty(context.Background())
	if err!=nil {
		InternalServerError(w, errors.New("something went wrong"))
		return
	}
	var request createUserRequest
	err = json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		BadRequestError(w)
		return
	}
	var createdUser model.User
	if usersEmpty {
		createdUser, err = createUserHelper(request, "admin")
	}else {
		adminUser := tools.RequirePermission(w, *r, 1) // user has to be admin
		if adminUser == nil {
			return
		}
		createdUser, err = createUserHelper(request, "lecturer")
	}
	if err != nil {
		BadRequestError(w)
		return
	}
	writeJSON(context.Background(), w, createUserResponse{Name: createdUser.Name, Email: createdUser.Email, Role: createdUser.Role})
}

func createUserHelper(request createUserRequest, userType string) (user model.User, err error) {
	var u = model.User{
		Name:  request.Name,
		Email: request.Email,
		Role:  userType,
	}
	if userType == "admin" {
		err = u.SetPassword(request.Password)
		if err != nil {
			return u, errors.New("user could not be created")
		}
	}
	if !u.ValidateFields() {
		return u, errors.New("user data rejected")
	}
	dbErr := dao.CreateUser(context.Background(), u)
	if dbErr != nil {
		return u, errors.New("user could not be created")
	}
	if userType != "admin" { //generate password set link and send out email
		err = forgotPassword(request.Email)
	}
	return u, nil
}

func forgotPassword(email string) error {
	u, err := dao.GetUserByEmail(context.Background(), email)
	if err != nil {
		log.Printf("couldn't get user by email")
		return err
	}
	registerLink, err := dao.CreateRegisterLink(context.Background(), u)
	if err != nil {
		log.Printf("couldn't create register link\n")
		return err
	}
	log.Printf("register link: %v\n", registerLink)
	err = tools.SendPasswordMail(email, registerLink.RegisterSecret)
	return err
}

type createUserRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type createUserResponse struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  string `json:"role"`
}
