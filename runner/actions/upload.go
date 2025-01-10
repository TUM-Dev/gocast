package actions

import (
	"context"
	"github.com/TUM-Dev/gocast/worker/cfg"
	"github.com/tum-dev/gocast/runner/protobuf"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"
)

func (a *ActionProvider) UploadAction() *Action {
	return &Action{
		Type: UploadAction,
		ActionFn: func(ctx context.Context, log *slog.Logger) (context.Context, error) {

			//course := ctx.Value("course").(uint32)
			stream := ctx.Value("stream").(uint32)
			url := ctx.Value("url").(string)

			file := ctx.Value("uploadFile").(string)

			//this is the part that from worker/upload.go
			client := &http.Client{
				Timeout: time.Minute * 15,
			}
			r, w := io.Pipe()
			writer := multipart.NewWriter(w)

			go func() {
				defer w.Close()
				defer writer.Close()
				formFileWriter, err := writer.CreateFormFile("filename", file)
				if err != nil {
					log.Error("cannot create form file: ", err)
					return
				}
				fileReader, err := os.Open(file)
				if err != nil {
					log.Error("cannot create form file: ", err)
					return
				}
				defer fileReader.Close()
				_, err = io.Copy(formFileWriter, fileReader)
				if err != nil {
					log.Error("cannot create form file: ", err)
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
					formFieldWriter, err := writer.CreateFormField(name)
					if err != nil {
						log.Error("Cannot create form field: ", err)
						return
					}
					_, err = io.Copy(formFieldWriter, strings.NewReader(value))
					if err != nil {
						log.Error("Cannot create form field: ", err)
						return
					}
				}
			}()
			rsp, err := client.Post(cfg.UploadUrl, writer.FormDataContentType(), r)
			if err == nil && rsp.StatusCode != http.StatusOK {
				log.Error("Request failed with response code: ", rsp.StatusCode)
			}
			if err == nil && rsp != nil {
				all, err := io.ReadAll(rsp.Body)
				if err == nil {
					log.Info("File got uploaded", "Uploaded file", file)
					log.Debug("file", all)
				}
			}
			a.Server.NotifyVoDUploadFinished(ctx, &protobuf.VoDUploadFinished{
				HLSUrl:       url,
				StreamID:     stream,
				RunnerID:     "",
				SourceType:   "",
				ThumbnailUrl: "",
			})
			return ctx, nil
		},
	}
}
