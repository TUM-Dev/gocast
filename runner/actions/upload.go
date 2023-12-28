package actions

import (
	"context"
	"fmt"
	"github.com/TUM-Dev/gocast/worker/cfg"
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

			streamID, ok := ctx.Value("stream").(uint64)
			if !ok {
				return ctx, fmt.Errorf("%w: context doesn't contain stream", ErrRequiredContextValNotFound)
			}
			courseID, ok := ctx.Value("course").(uint64)
			if !ok {
				return ctx, fmt.Errorf("%w: context doesn't contain courseID", ErrRequiredContextValNotFound)
			}
			version, ok := ctx.Value("version").(string)
			if !ok {
				return ctx, fmt.Errorf("%w: context doesn't contain version", ErrRequiredContextValNotFound)
			}

			URLstring := ctx.Value("URL").(string)

			fileName := fmt.Sprintf("%s/%s/%s/%s.mp4", a.MassDir, courseID, streamID, version)

			client := &http.Client{
				// 5 minutes timeout, some large files can take a while.
				Timeout: time.Minute * 5,
			}

			r, w := io.Pipe()
			writer := multipart.NewWriter(w)

			//the same function as in the worker but without function calling
			//so analyzing it and changing it later won't give much to look through

			go func() {
				defer func(w *io.PipeWriter) {
					err := w.Close()
					if err != nil {

					}
				}(w)
				defer func(writer *multipart.Writer) {
					err := writer.Close()
					if err != nil {

					}
				}(writer)
				formFileWriter, err := writer.CreateFormFile("filename", fileName)
				if err != nil {
					log.Error("Cannot create form file: ", err)
					return
				}
				FileReader, err := os.Open(fileName)
				if err != nil {
					log.Error("Cannot create form file: ", err)
					return
				}
				defer func(FileReader *os.File) {
					err := FileReader.Close()
					if err != nil {

					}
				}(FileReader)
				_, err = io.Copy(formFileWriter, FileReader)
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
					formFileWriter, err := writer.CreateFormField(name)
					if err != nil {
						log.Error("Cannot create form field: ", err)
						return
					}
					_, err = io.Copy(formFileWriter, strings.NewReader(value))
					if err != nil {
						log.Error("Cannot create form field: ", err)
						return
					}
					if err != nil {
						log.Error("Cannot create form field: ", err)
						return
					}
				}
			}()
			rsp, err := client.Post(URLstring, writer.FormDataContentType(), r)
			if err == nil && rsp.StatusCode != http.StatusOK {
				log.Error("Request failed with response code: ", rsp.StatusCode)
			}
			if err == nil && rsp != nil {
				all, err := io.ReadAll(rsp.Body)
				if err == nil {
					log.Debug(string(all), "fileUploaded", fileName)
				}
			}
			if err != nil {
				log.Error("Failed to post video to TUMLive", "error", err)
				return ctx, err
			}
			log.Info("Successfully posted video to TUMLive", "stream", fileName)

			return ctx, err
		},
	}
}
