FROM amd64/golang:1.20-alpine3.17 as builder

WORKDIR /go/src/github.com/joschahenningsen/TUM-Live/worker
COPY . .

RUN apk update && \
    apk add build-base && \
    apk add tesseract-ocr && \
    apk add tesseract-ocr-dev

RUN GO111MODULE=on go mod download && go mod verify
# bundle version into binary if specified in build-args, dev otherwise.
ARG version=dev
RUN GOOS=linux GOARCH=amd64 go build -a -ldflags "-w -X main.VersionTag=${version}" -o /worker cmd/worker/worker.go

FROM alpine:3.17

RUN apk add --no-cache \
  ffmpeg \
  tzdata

# copy shared libraries for tesseract
COPY --from=builder \
    /usr/lib/liblept*.so.* /usr/lib/libtesseract*.so.* \
    /usr/lib/libgif*.so.*  /usr/lib/libtiff*.so.* \
    /usr/lib/libzstd*.so.* /usr/lib/liblzma*.so.* /usr/lib/
COPY --from=builder /worker /worker
RUN chmod +x /worker

CMD ["/worker"]
