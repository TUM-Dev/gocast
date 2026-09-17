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
# packages just because a .ts/.vue file changed. The postinstall build scripts need
# the real source tree, so skip them here and run them explicitly once it's copied in.
COPY web/package.json web/pnpm-lock.yaml web/.npmrc web/
COPY frontend/package.json frontend/pnpm-lock.yaml frontend/
RUN --mount=type=cache,target=/root/.local/share/pnpm/store,sharing=locked \
    cd web && pnpm install --frozen-lockfile --ignore-scripts
RUN --mount=type=cache,target=/root/.local/share/pnpm/store,sharing=locked \
    cd frontend && pnpm install --frozen-lockfile --ignore-scripts

COPY web web
COPY frontend frontend

## remove generated files in case the developer built locally before
RUN rm -rf web/assets/ts-dist web/assets/css-dist web/assets/vendor &&\
    rm -rf web/spa/assets web/spa/index.html

# Not --prod: the postinstall that bundles the templates' JS and CSS runs webpack and
# the tailwind CLI, both devDependencies.
RUN cd web && pnpm run build && pnpm run tailwind-compile

## build the single-page app serving the migrated pages; output lands in web/spa
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
COPY --from=node /app/web/assets ./web/assets
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
