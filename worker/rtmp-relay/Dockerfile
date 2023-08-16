FROM golang:1.20 as authBuilder

COPY main.go .
RUN CGO_ENABLED=0 go build -ldflags="-extldflags=-static" -o /auth main.go

FROM aler9/rtsp-simple-server:v1.0.0 AS rtsp

FROM alpine:3.18

COPY --from=rtsp /rtsp-simple-server /
COPY --from=authBuilder /auth /auth

WORKDIR /
RUN apk add file
ADD rtsp-simple-server.yml /rtsp-simple-server.yml
ADD entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

CMD [ "/entrypoint.sh" ]
