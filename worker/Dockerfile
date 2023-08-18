FROM amd64/golang:1.21-alpine3.18 as builder

WORKDIR /go/src/github.com/joschahenningsen/TUM-Live/worker
COPY . .

RUN GO111MODULE=on go mod download
# bundle version into binary if specified in build-args, dev otherwise.
ARG version=dev
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -ldflags "-w -extldflags '-static' -X main.V=${version}" -o /worker cmd/main.go

FROM bluenviron/mediamtx:latest as mediamtx

FROM alpine:3.18
ADD entrypoint.sh /entrypoint.sh
ADD mediamtx.yml /mediamtx.yml
RUN chmod +x /entrypoint.sh

RUN apk add --no-cache \
  ffmpeg \
  tzdata

COPY --from=builder /worker /worker
RUN chmod +x /worker
COPY --from=mediamtx /mediamtx /mediamtx
RUN chmod +x /mediamtx

CMD ["/entrypoint.sh"]