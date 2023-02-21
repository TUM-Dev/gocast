FROM golang:1.20-alpine3.17 as builder

WORKDIR /app

COPY . .

RUN go build -o /vod-service cmd/vod-service/main.go

FROM alpine:3.17

RUN apk add ffmpeg
COPY --from=builder /vod-service /vod-service

EXPOSE 8089

CMD ["/vod-service"]