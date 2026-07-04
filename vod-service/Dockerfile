FROM golang:1.21-alpine3.18 as builder

WORKDIR /app

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /vod-service cmd/vod-service/main.go

FROM alpine:3.18

RUN apk add ffmpeg
COPY --from=builder /vod-service /vod-service

EXPOSE 8089

CMD ["/vod-service"]