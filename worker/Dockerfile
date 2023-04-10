FROM amd64/golang:1.20-alpine3.17 as builder

WORKDIR /go/src/github.com/joschahenningsen/TUM-Live/worker
COPY . .

RUN GO111MODULE=on go mod download
# bundle version into binary if specified in build-args, dev otherwise.
ARG version=dev
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -ldflags "-w -extldflags '-static' -X main.VersionTag=${version}" -o /worker cmd/worker/worker.go

FROM alpine:3.17

RUN apk add --no-cache \
  ffmpeg \
  tzdata

COPY --from=builder /worker /worker
RUN chmod +x /worker

CMD ["/worker"]