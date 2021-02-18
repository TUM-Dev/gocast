FROM golang:1.16

WORKDIR /go/src/app
COPY . .

RUN go install ./app/server/

RUN wget -O /bin/wait-for-it.sh https://raw.githubusercontent.com/vishnubob/wait-for-it/master/wait-for-it.sh
RUN chmod +x /bin/wait-for-it.sh

# wait for database to fully start before starting backend
CMD ["wait-for-it.sh", "db:5432", "--", "server"]
