package worker

import (
	"github.com/TUM-Dev/gocast/worker/cfg"
	"net/http"
	"os"
	"sync"
	"testing"
)

func TestUpload(t *testing.T) {
	cfg.LrzUser = "TestUserName"
	cfg.LrzMail = "mail@mail.de"
	cfg.LrzPhone = "0123456789"
	cfg.LrzSubDir = "testDir"
	cfg.LrzUploadUrl = "http://localhost:8080/"
	var filesize uint = 1024 // mB
	filename, err := createDummyFile(filesize)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(filename)

	go post(filename)
	var finished sync.WaitGroup
	finished.Add(1)
	handler := http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		defer finished.Done()
		err := request.ParseMultipartForm(32 << 20)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			t.Fatal(err)
			return
		}
		if is := request.Form["benutzer"][0]; is != cfg.LrzUser {
			t.Fatalf("benutzer:  %v != %v", is, cfg.LrzUser)
		}
		if is := request.Form["mailadresse"][0]; is != cfg.LrzMail {
			t.Fatalf("mailadresse:  %v != %v", is, cfg.LrzMail)
		}
		if is := request.Form["telefon"][0]; is != cfg.LrzPhone {
			t.Fatalf("telefon:  %v != %v", is, cfg.LrzPhone)
		}
		if is := request.Form["unidir"][0]; is != "tum" {
			t.Fatalf("unidir:  %v != tum", is)
		}
		if is := request.Form["subdir"][0]; is != cfg.LrzSubDir {
			t.Fatalf("subdir:  %v != %v", is, cfg.LrzSubDir)
		}

		_, h, err := request.FormFile("filename")
		if err != nil {
			t.Fatal(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if uint(h.Size) != filesize*1<<20 {
			t.Fatalf("incorrect size: %v", h.Size)
		}
		w.WriteHeader(200)
	})
	srv := http.Server{Addr: ":8080"}
	defer srv.Close()
	http.Handle("/", handler)
	go srv.ListenAndServe()
	finished.Wait()
}

func createDummyFile(filesize uint) (string, error) {
	file, err := os.CreateTemp("/tmp", "recording")
	if err != nil {
		return "", err
	}
	f, err := os.OpenFile(file.Name(), os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return "", err
	}
	defer f.Close()
	data := make([]byte, 1<<20)
	for i := uint(0); i < filesize; i++ {
		if _, err = f.Write(data); err != nil {
			return "", err
		}
	}
	return file.Name(), nil
}
