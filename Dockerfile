FROM node:18 as node

WORKDIR /app
COPY web web

## remove generated files in case the developer build with npm before
RUN rm -rf web/assets/ts-dist
RUN rm -rf web/assets/css-dist

WORKDIR /app/web
RUN npm i --no-dev

FROM golang:1.19-alpine as build-env
RUN mkdir /gostuff
WORKDIR /gostuff
COPY go.mod .
COPY go.sum .

# Get dependencies - will also be cached if we won't change mod/sum
RUN go mod download

WORKDIR /go/src/app
COPY . .
COPY --from=node /app/web/assets ./web/assets
COPY --from=node /app/web/node_modules ./web/node_modules

# bundle version into binary if specified in build-args, dev otherwise.
ARG version=dev
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-w -extldflags '-static' -X main.VersionTag=${version}" -o /go/bin/tumlive cmd/tumlive/tumlive.go

FROM alpine:3.16.2
RUN apk add --no-cache tzdata
WORKDIR /app
COPY --from=build-env /go/bin/tumlive .
CMD ["sh", "-c", "sleep 3 && ./tumlive"]
