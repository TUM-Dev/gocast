FROM golang:1.27 AS builder

WORKDIR /go/src/github.com/TUM-Dev/gocast/edge-service
COPY . .

RUN GO111MODULE=on go mod download

# bundle version into binary if specified in build-args, dev otherwise.
ARG version=dev
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -ldflags "-w -extldflags '-static' -X main.VersionTag=${version}" -o /edge-service *.go &&\
    chmod +x /edge-service

FROM alpine:3.24
COPY --from=builder /edge-service /edge-service

RUN apk add --no-cache ffmpeg

CMD ["/edge-service"]
