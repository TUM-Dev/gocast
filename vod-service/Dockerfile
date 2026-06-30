FROM golang:1.26 AS builder

WORKDIR /app

COPY . .

RUN CGO_ENABLED=0 go build -o /vod-service cmd/vod-service/main.go

FROM alpine:3.24

RUN apk add ffmpeg
COPY --from=builder /vod-service /vod-service

EXPOSE 8089

CMD ["/vod-service"]
