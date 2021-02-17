FROM golang:1.16

WORKDIR /go/src/app
COPY . .

RUN make app

CMD ["bin/app"]
