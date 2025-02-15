FROM amd64/golang:1.21-alpine3.18 as builder

WORKDIR /go/src/github.com/TUM-Dev/gocast/runner
COPY . .

RUN GO111MODULE=on go mod download
# bundle version into binary if specified in build-args, dev otherwise.
ARG version=dev
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -ldflags "-w -extldflags '-static' -X main.V=${version}" -o /runner cmd/runner/main.go

FROM bluenviron/mediamtx:1.1.0 as mediamtx

FROM alpine:3.18
ADD entrypoint.sh /entrypoint.sh
ADD mediamtx.yml /mediamtx.yml
RUN chmod +x /entrypoint.sh

RUN apk add --no-cache \
  ffmpeg \
  tzdata

COPY --from=builder /runner /runner
RUN chmod +x /runner
COPY --from=mediamtx /mediamtx /mediamtx
RUN chmod +x /mediamtx

CMD ["/entrypoint.sh"]