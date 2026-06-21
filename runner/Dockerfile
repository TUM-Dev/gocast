FROM golang:1.26 AS builder

WORKDIR /go/src/github.com/TUM-Dev/gocast/runner
COPY . .

RUN GO111MODULE=on go mod download
# bundle version into binary if specified in build-args, dev otherwise.
ARG version=dev
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -ldflags "-w -extldflags '-static' -X main.V=${version}" -o /runner cmd/runner/main.go

FROM alpine:3.24

RUN apk add --no-cache \
  ffmpeg \
  tzdata

COPY --from=builder /runner /runner
RUN chmod +x /runner

CMD ["/runner"]
