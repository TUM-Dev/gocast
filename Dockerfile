FROM node:15.9.0 as node

WORKDIR /app
COPY . .

## remove generated files in case the developer build with npm before
RUN rm -rf web/assets/ts-dist
RUN rm -rf web/assets/css-dist

RUN npm i --no-dev

FROM golang:1.16

WORKDIR /go/src/app
COPY . .

COPY --from=node /app .

RUN go install ./app/server/

#RUN wget -O /bin/wait-for-it.sh https://raw.githubusercontent.com/vishnubob/wait-for-it/master/wait-for-it.sh
#RUN chmod +x /bin/wait-for-it.sh

# wait for database to fully start before starting backend
#CMD ["wait-for-it.sh", "db:3306", "--", "server"]
CMD ["bash", "-c", "sleep 3 && server"]
