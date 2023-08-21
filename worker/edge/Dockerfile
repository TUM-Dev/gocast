FROM golang:1.21-alpine3.18 as builder

WORKDIR /go/src/github.com/joschahenningsen/TUM-Live/worker
COPY . .

RUN GO111MODULE=on go mod download

# bundle version into binary if specified in build-args, dev otherwise.
ARG version=dev
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -ldflags "-w -extldflags '-static' -X main.VersionTag=${version}" -o /worker *.go &&\
    chmod +x /worker

FROM alpine:3.18
COPY --from=builder /worker /worker

RUN apk add --no-cache ffmpeg

CMD ["/worker"]
