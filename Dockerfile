# syntax=docker/dockerfile:1

FROM node:25 AS node

# Node 25 dropped the bundled corepack, and the image has no pnpm. Installing corepack
# rather than pnpm directly keeps packageManager in each package.json the single place
# the pnpm version is pinned. The env var stops corepack prompting before it fetches.
ENV COREPACK_ENABLE_DOWNLOAD_PROMPT=0
RUN npm install -g corepack@latest --force && corepack enable pnpm

WORKDIR /app

# Install deps from the lockfiles alone first, so this layer - and the pnpm store
# cache mount behind it - survives source-only changes and doesn't redownload
# packages just because a .ts/.vue file changed.
COPY frontend/package.json frontend/pnpm-lock.yaml frontend/
RUN --mount=type=cache,target=/root/.local/share/pnpm/store,sharing=locked \
    cd frontend && pnpm install --frozen-lockfile --ignore-scripts

# Only web/spa is needed from web/: it is where the build writes.
COPY web/spa web/spa
COPY frontend frontend

## remove generated files in case the developer built locally before
RUN rm -rf web/spa/assets web/spa/index.html

## build the single-page app; output lands in web/spa
RUN cd frontend && pnpm run build

FROM golang:1.27 AS build-env

RUN mkdir /gostuff
WORKDIR /gostuff
COPY go.mod go.sum ./

# Get dependencies - will also be cached if we won't change mod/sum
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

WORKDIR /go/src/app
COPY . .
COPY --from=node /app/web/spa ./web/spa

# bundle version into binary if specified in build-args, dev otherwise.
ARG version=dev
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-w -extldflags '-static' -X main.VersionTag=${version}" -o /go/bin/tumlive cmd/tumlive/main.go

FROM alpine:3.24
RUN apk add --no-cache tzdata openssl
WORKDIR /app
COPY --from=build-env /go/bin/tumlive .
CMD ["sh", "-c", "sleep 3 && ./tumlive"]
