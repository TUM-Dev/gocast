FROM golang:1.16

WORKDIR /go/src/app
COPY . .

RUN go install ./app/server/

CMD ["server"]
