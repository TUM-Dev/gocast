FROM node:25 AS node

WORKDIR /app
COPY web web
COPY frontend frontend

## remove generated files in case the developer build with npm before
RUN rm -rf web/assets/ts-dist &&\
    rm -rf web/assets/css-dist &&\
    rm -rf web/spa/assets web/spa/index.html

WORKDIR /app/web
RUN npm i --no-dev

## build the single-page app serving the migrated pages; output lands in web/spa
WORKDIR /app/frontend
RUN npm ci && npm run build

FROM golang:1.26 AS build-env

RUN mkdir /gostuff
WORKDIR /gostuff
COPY go.mod go.sum ./

# Get dependencies - will also be cached if we won't change mod/sum
RUN go mod download

WORKDIR /go/src/app
COPY . .
COPY --from=node /app/web/assets ./web/assets
COPY --from=node /app/web/node_modules ./web/node_modules
COPY --from=node /app/web/spa ./web/spa

# bundle version into binary if specified in build-args, dev otherwise.
ARG version=dev
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-w -extldflags '-static' -X main.VersionTag=${version}" -o /go/bin/tumlive cmd/tumlive/main.go

FROM alpine:3.24
RUN apk add --no-cache tzdata openssl
WORKDIR /app
COPY --from=build-env /go/bin/tumlive .
CMD ["sh", "-c", "sleep 3 && ./tumlive"]
