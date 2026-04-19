FROM golang:1.26 AS builder


WORKDIR /go/src/github.com/TUM-Dev/gocast/worker
COPY . .

RUN GO111MODULE=on go mod download
# bundle version into binary if specified in build-args, dev otherwise.
ARG version=dev
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -ldflags "-w -extldflags '-static' -X main.VersionTag=${version}" -o /worker cmd/worker/worker.go

FROM bluenviron/mediamtx:latest AS rtsp

FROM alpine:3.23
ADD entrypoint.sh /entrypoint.sh
ADD mediamtx.yml /mediamtx.yml
RUN chmod +x /entrypoint.sh

RUN apk add --no-cache \
  ffmpeg \
  tzdata

COPY --from=builder /worker /worker
RUN chmod +x /worker
COPY --from=rtsp /mediamtx /mediamtx
RUN chmod +x /mediamtx

CMD ["/entrypoint.sh"]
