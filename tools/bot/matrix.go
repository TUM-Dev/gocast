package bot

import (
	"TUM-Live/tools"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
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
const loginUrl = clientUrl + "login"
const maxID = 10000

// SendBotMessage sends a formatted message to a matrix room
func (ma *Matrix) SendBotMessage(info InfoMessage) error {
	id := strconv.Itoa(rand.Intn(maxID)) // transaction id
	authToken, err := getAuthToken()
	if err != nil {
		return err
	}
	if authToken == "" {
		return errors.New("authentication failed, could not get token")
	}

	roomMessageSuffix = "/send/m.room.message/" + id + "?access_token="
	url := clientUrl + "rooms/" + tools.Cfg.Alerts.Matrix.RoomID + roomMessageSuffix + authToken
	matrixMessage := Message{
		MsgType:       "m.text",
		Body:          generateInfoText(info),
		Format:        "org.matrix.custom.html",
		FormattedBody: getFormattedMessageText(info),
	}
	matrixMessageJSON, err := json.Marshal(matrixMessage)
	if err != nil {
		return err
	}
	err = sendMessageRequest(url, bytes.NewBuffer(matrixMessageJSON))
	return err
}

func sendMessageRequest(url string, body io.Reader) error {
	client := &http.Client{}
	request, err := http.NewRequest(http.MethodPut, url, body)
	if err != nil {
		return err
	}
	response, err := client.Do(request)
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf(fmt.Sprintf("received status code %d instead of %d.", response.StatusCode, http.StatusOK))
	}
	return err
}

// getAuthToken retrieves a single use token for the next message sent to the server.
func getAuthToken() (string, error) {
	login := Login{
		Type:     "m.login.password",
		User:     tools.Cfg.Alerts.Matrix.Username,
		Password: tools.Cfg.Alerts.Matrix.Password,
	}
	loginRequest, err := json.Marshal(login)
	if err != nil {
		return "", err
	}
	response, err := http.Post(loginUrl, "application/json", bytes.NewBuffer(loginRequest))
	if err != nil {
		return "", err
	}
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf(fmt.Sprintf("received status code %d instead of %d.", response.StatusCode, http.StatusOK))
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

	return loginResponse.AccessToken, err
}
