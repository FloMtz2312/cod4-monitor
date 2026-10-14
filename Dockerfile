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

# The demo viewer cod4-demo (github.com/TiPSYSPiT/cod4-demo, GPL-3.0), served on /demo-viewer/: plain HTML/JS that
# parses a demo in the browser. Pinned to a commit; the map geometry (119 MB, only for the "unseen" kill column)
# is left out unless DEMO_GEOMETRY=1.
FROM alpine:3.22 AS demoviewer
ARG COD4_DEMO_REF=34f9cc265980febe46886d20e2c69abc494a645e
ARG DEMO_GEOMETRY=0
ADD https://github.com/TiPSYSPiT/cod4-demo/archive/${COD4_DEMO_REF}.tar.gz /tmp/viewer.tgz
RUN mkdir /viewer && tar -xzf /tmp/viewer.tgz -C /viewer --strip-components 1 && rm /tmp/viewer.tgz \
    && cd /viewer && rm -rf python tools docs start.bat .github .gitignore \
    && if [ "$DEMO_GEOMETRY" != 1 ]; then rm -rf geometry; fi

FROM alpine:3.22
RUN apk add --no-cache tzdata && adduser -D -u 1000 app && mkdir /data && chown app /data
USER app
COPY --from=build /monitor /monitor
COPY --from=demoviewer /viewer /demo-viewer
ENV DB_PATH=/data/monitor.db DEMO_VIEWER_DIR=/demo-viewer
EXPOSE 8080
ENTRYPOINT ["/monitor"]
