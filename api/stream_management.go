package api

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/julienschmidt/httprouter"
	"net/http"
	"strings"
)

func configGinStreamAuthRouter(router gin.IRoutes) {
	router.POST("/on_publish", ConverHttprouterToGin(AuthenticateStream))
	router.POST("/on_publish_done", ConverHttprouterToGin(EndStream))
	router.POST("/on_record_done", ConverHttprouterToGin(OnRecordingFinished))
}

/**
* This function is called when a user attempts to push a stream to the server.
* @w: response writer. Status code determines wether streaming is approved: 200 if yes, 402 otherwise.
* @r: request. Form if valid: POST /on_publish/app/kurs-key example: {/on_publish/eidi-3zt45z452h4754nj2q74}
 */
func AuthenticateStream(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	w.WriteHeader(200)
}

func EndStream(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

}

// TODO: Convert recording to mp4 and put into correct directory. Delete flv file.
func OnRecordingFinished(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	println(formatRequest(r))
}

func formatRequest(r *http.Request) string {
	// Create return string
	var request []string
	// Add the request string
	url := fmt.Sprintf("%v %v %v", r.Method, r.URL, r.Proto)
	request = append(request, url)
	// Add the host
	request = append(request, fmt.Sprintf("Host: %v", r.Host))
	// Loop through headers
	for name, headers := range r.Header {
		name = strings.ToLower(name)
		for _, h := range headers {
			request = append(request, fmt.Sprintf("%v: %v", name, h))
		}
	}

	// If this is a POST, add post data
	if r.Method == "POST" {
		r.ParseForm()
		request = append(request, "\n")
		request = append(request, r.Form.Encode())
	}
	// Return the request as a string
	return strings.Join(request, "\n")
}
