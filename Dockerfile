# The v2 interface (ui/, Svelte): built first, then embedded in the Go binary.
FROM node:22-alpine AS ui
WORKDIR /ui
COPY ui/package.json ui/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci --no-audit --no-fund
COPY ui/ ./
RUN npm run build

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
COPY --from=ui /ui/dist ./ui/dist
# The compiled packages (SQLite above all) are kept between builds: only the changed code is recompiled.
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    go test ./... && CGO_ENABLED=0 go build -ldflags="-s -w" -o /monitor ./cmd/monitor

FROM alpine:3.22
RUN apk add --no-cache tzdata && adduser -D -u 1000 app && mkdir /data && chown app /data
USER app
COPY --from=build /monitor /monitor
ENV DB_PATH=/data/monitor.db
EXPOSE 8080
ENTRYPOINT ["/monitor"]
