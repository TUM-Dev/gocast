package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// AuthReq is the request body sent by rtsp-simple-server to the auth app
type AuthReq struct {
	Ip       string      `json:"ip"`
	User     string      `json:"user"`
	Password string      `json:"password"`
	Path     string      `json:"path"`
	Protocol string      `json:"protocol"`
	Id       interface{} `json:"id"`
	Action   string      `json:"action"`
	Query    string      `json:"query"`
}

// this is an authentication app that replies with 200 if the publishing path is in the VALID_PATHS env var, >= 4ßß otherwise
func main() {
	validPaths := strings.Split(os.Getenv("VALID_PATHS"), ",")
	fmt.Printf("Valid paths: %s\n", validPaths)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var req AuthReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			fmt.Println(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		for _, s := range validPaths {
			if s == req.Path {
				w.WriteHeader(http.StatusOK)
				return
			}
		}
		w.WriteHeader(http.StatusForbidden)
	})

	fmt.Println(http.ListenAndServe("127.0.0.1:9999", nil).Error())
}
