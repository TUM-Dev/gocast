package tools

import (
	"TUM-Live/model"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
)

type Message struct {
	MsgType string `json:"msgtype"`
	Body    string `json:"body"`
}

type Login struct {
	Type     string `json:"type"`
	User     string `json:"user"`
	Password string `json:"password"`
}

type Response struct {
	EventID string `json:"event_id"`
}

type LoginResponse struct {
	UserId      string `json:"user_id"`
	AccessToken string `json:"access_token"`
	HomeServer  string `json:"home_server"`
	DeviceId    string `json:"device_id"`
	WellKnown   struct {
		MHomeserver struct {
			BaseUrl string `json:"base_url"`
		} `json:"m.homeserver"`
	} `json:"well_known"`
}

type BotMessage struct {
	CourseName  string
	LectureHall string
	StreamUrl   string
	CombIP      string
	CameraIP    string
}

const urlSuffix string = "/send/m.room.message/123?access_token="

var clientUrl = "https://matrix.org/_matrix/client/r0/"
var urlPrefix = clientUrl + "rooms/"
var loginUrl = "https://matrix.org/_matrix/client/r0/login"

func SendBotMessage(stream model.Stream, course model.Course, lectureHall model.LectureHall) error {
	// Currently, only supported for lecture hall streams
	if stream.LectureHallID == 0 {
		return nil
	}

	botMessage := BotMessage{
		CourseName:  course.Name,
		LectureHall: lectureHall.Name,
		StreamUrl:   Cfg.WebUrl + "/w/" + course.Slug + "/" + strconv.Itoa(int(stream.ID)),
		CombIP:      lectureHall.CombIP,
		CameraIP:    lectureHall.CameraIP,
	}

	authToken, err := getAuthToken()
	if err != nil {
		panic(err)
	}

	sendMessage(authToken, botMessage)
	return nil
}

func getMessageText(botMessage BotMessage) string {
	return "Technical problem:\n" +
		"URL: " + botMessage.StreamUrl + "\n" +
		"Course name: " + botMessage.CourseName + "\n" +
		"Lecture: " + botMessage.LectureHall + "\n" +
		"CombinedIP: " + botMessage.CombIP + "\n" +
		"CameraIP: " + botMessage.CameraIP + "\n"
}

func sendMessage(accessToken string, botMessage BotMessage) error {
	client := &http.Client{}

	message := Message{
		MsgType: "m.text",
		Body:    getMessageText(botMessage),
	}

	m, err := json.Marshal(message)
	if err != nil {
		panic(err)
	}

	url := urlPrefix + Cfg.Matrix.RoomID + urlSuffix + accessToken

	req, err := http.NewRequest(http.MethodPut,
		url,
		bytes.NewBuffer(m))

	if err != nil {
		panic(err)
	}

	_, err = client.Do(req)
	if err != nil {
		panic(err)
	}
	return nil
}

func getAuthToken() (string, error) {
	client := &http.Client{}

	login := Login{
		Type:     "m.login.password",
		User:     Cfg.Matrix.Username,
		Password: Cfg.Matrix.Password,
	}

	loginRequest, err := json.Marshal(login)
	if err != nil {
		panic(err)
	}

	request, err := http.NewRequest(
		http.MethodPost,
		loginUrl,
		bytes.NewBuffer(loginRequest),
	)

	if err != nil {
		panic(err)
	}

	response, err := client.Do(request)

	parsedRequest, err := io.ReadAll(response.Body)
	if err != nil {
		panic(err)
	}

	loginResponse := LoginResponse{}
	err = json.Unmarshal(parsedRequest, &loginResponse)
	if err != nil {
		panic(err)
	}

	return loginResponse.AccessToken, nil
}
