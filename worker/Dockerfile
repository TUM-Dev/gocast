FROM golang:1.18-alpine3.15 as builder

WORKDIR /go/src/github.com/joschahenningsen/TUM-Live/worker
COPY . .

RUN GO111MODULE=on go mod download
# bundle version into binary if specified in build-args, dev otherwise.
ARG version=dev
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -ldflags "-w -extldflags '-static' -X main.VersionTag=${version}" -o /worker cmd/worker/worker.go

FROM alpine:3.16

RUN apk add --no-cache \
  ffmpeg=4.4.1-r2 \
  tzdata=2022a-r0

COPY --from=builder /worker /worker
RUN chmod +x /worker

CMD ["/worker"]
