package worker

import (
	"github.com/joschahenningsen/TUM-Live/worker/cfg"
	log "github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"
)

func upload(streamCtx *StreamContext) {
	log.WithField("stream", streamCtx.getStreamName()).Info("Uploading stream")
	err := post(streamCtx.getTranscodingFileName())
	if err != nil {
		log.WithField("stream", streamCtx.getStreamName()).WithError(err).Error("Error uploading stream")
	}
	log.WithField("stream", streamCtx.getStreamName()).Info("Uploaded stream")
}

func post(file string) error {
	client := &http.Client{
		// 5 minutes timeout, some large files can take a while.
		Timeout: time.Minute * 15,
	}
	r, w := io.Pipe()
	writer := multipart.NewWriter(w)

	go func() {
		defer w.Close()
		defer writer.Close()
		err := writeFile(writer, "filename", file)
		if err != nil {
			log.Error("Cannot create form file: ", err)
			return
		}

		fields := map[string]string{
			"benutzer":    cfg.LrzUser,
			"mailadresse": cfg.LrzMail,
			"telefon":     cfg.LrzPhone,
			"unidir":      "tum",
			"subdir":      cfg.LrzSubDir,
			"info":        "",
		}

		for name, value := range fields {
			err = writeField(writer, name, value)
			if err != nil {
				log.Error("Cannot create form field: ", err)
				return
			}
		}
	}()
	rsp, err := client.Post(cfg.LrzUploadUrl, writer.FormDataContentType(), r)
	if err == nil && rsp.StatusCode != http.StatusOK {
		log.Error("Request failed with response code: ", rsp.StatusCode)
	}
	if err == nil && rsp != nil {
		all, err := ioutil.ReadAll(rsp.Body)
		if err == nil {
			log.WithField("fileUploaded", file).Debug(string(all))
		}
	}
	return err
}

func writeField(writer *multipart.Writer, name string, value string) error {
	formFieldWriter, err := writer.CreateFormField(name)
	if err != nil {
		return err
	}
	_, err = io.Copy(formFieldWriter, strings.NewReader(value))
	return err
}

func writeFile(writer *multipart.Writer, fieldname string, file string) error {
	formFileWriter, err := writer.CreateFormFile(fieldname, file)
	if err != nil {
		return err
	}
	fileReader, err := os.Open(file)
	if err != nil {
		return err
	}
	defer fileReader.Close()
	_, err = io.Copy(formFileWriter, fileReader)
	return err
}
