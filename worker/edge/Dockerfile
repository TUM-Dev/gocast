FROM golang:1.18-alpine3.15 as builder

WORKDIR /go/src/github.com/joschahenningsen/TUM-Live/worker
COPY . .

RUN GO111MODULE=on go mod download

# bundle version into binary if specified in build-args, dev otherwise.
ARG version=dev
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -ldflags "-w -extldflags '-static' -X main.VersionTag=${version}" -o /worker edge.go
RUN chmod +x /worker

FROM scratch
COPY --from=builder /worker /worker

CMD ["/worker"]
