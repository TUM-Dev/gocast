package bot

import (
	"TUM-Live/tools"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math/rand"
	"net/http"
	"strconv"
)

// Matrix strategy
type Matrix struct {
}

// Message represents a Matrix message event that includes html formatting as specified
// in https://spec.matrix.org/v1.2/client-server-api/#mroommessage-msgtypes
type Message struct {
	MsgType       string `json:"msgtype"`
	Body          string `json:"body"`
	Format        string `json:"format"`
	FormattedBody string `json:"formatted_body"`
}

// Login represents a login request that is used for authentication as specified
// in https://spec.matrix.org/v1.2/client-server-api/#login
type Login struct {
	Type     string `json:"type"`
	User     string `json:"user"`
	Password string `json:"password"`
}

// MessageResponse is the response for a message in our case
type MessageResponse struct {
	EventID string `json:"event_id"`
}

// LoginResponse is the response for a login request
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

var roomMessageSuffix string

const clientUrl = "https://matrix.org/_matrix/client/r0/"
const loginUrl = "https://matrix.org/_matrix/client/r0/login"

// SendBotMessage sends a formatted message to a matrix room
func (ma *Matrix) SendBotMessage(info BotInfo) error {
	id := strconv.Itoa(rand.Intn(1000)) // transaction id
	roomMessageSuffix = "/send/m.room.message/" + id + "?access_token="
	authToken, err := getAuthToken()
	if err != nil {
		return err
	}
	if authToken == "" {
		return errors.New("authentication failed, could not get token")
	}

	client := &http.Client{}

	matrixMessage := Message{
		MsgType:       "m.text",
		Body:          getMessageText(info),
		Format:        "org.matrix.custom.html",
		FormattedBody: getFormattedMessageText(info),
	}

	m, err := json.Marshal(matrixMessage)
	if err != nil {
		return err
	}

	url := clientUrl + "rooms/" + tools.Cfg.Matrix.RoomID + roomMessageSuffix + authToken

	req, err := http.NewRequest(http.MethodPut, url, bytes.NewBuffer(m))

	if err != nil {
		return err
	}

	_, err = client.Do(req)
	if err != nil {
		return err
	}

	return nil
}

// getAuthToken retrieves a single use token for the next message sent to the server.
func getAuthToken() (string, error) {
	client := &http.Client{}

	login := Login{
		Type:     "m.login.password",
		User:     tools.Cfg.Matrix.Username,
		Password: tools.Cfg.Matrix.Password,
	}

	loginRequest, err := json.Marshal(login)
	if err != nil {
		return "", err
	}

	request, err := http.NewRequest(http.MethodPost, loginUrl, bytes.NewBuffer(loginRequest))
	if err != nil {
		return "", err
	}

	response, err := client.Do(request)
	if err != nil {
		return "", err
	}

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}

	loginResponse := LoginResponse{}
	err = json.Unmarshal(responseBody, &loginResponse)
	if err != nil {
		return "", err
	}

	return loginResponse.AccessToken, nil
}
