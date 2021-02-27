FROM node:15.10.0 as node

WORKDIR /app
COPY web web
COPY package.json .
COPY package-lock.json .
COPY tailwind.config.js .
COPY tsconfig.json .

## remove generated files in case the developer build with npm before
RUN rm -rf web/assets/ts-dist
RUN rm -rf web/assets/css-dist

RUN npm i --no-dev

FROM golang:1.16-alpine as build-env
RUN mkdir /gostuff
WORKDIR /gostuff
COPY go.mod .
COPY go.sum .

# Get dependancies - will also be cached if we won't change mod/sum
RUN go mod download

WORKDIR /go/src/app
COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /go/bin/server app/server/main.go

FROM alpine
WORKDIR /app
COPY --from=build-env /go/bin/server .
COPY --from=node /app/node_modules node_modules
COPY --from=node /app/web web
CMD ["sh", "-c", "sleep 3 && ./server"]
