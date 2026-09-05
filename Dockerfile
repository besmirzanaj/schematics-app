# syntax=docker/dockerfile:1
# Self-contained Skemat server image.
#
# The bundled sample dataset is baked in at build time (seed stage), so
# `docker run` just works. To serve the real production tree instead, bind-mount
# it over /data and run with --user "$(id -u):$(id -g)".

# ---- build: compile the pure-Go static server binary ----
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/skemat-server ./cmd/server

# ---- seed: ingest the bundled sample schematics into a fresh DB ----
FROM python:3-alpine AS seed
WORKDIR /src
COPY . .
RUN python3 ingest/ingest.py \
      --source sample/source \
      --dest /seed/live \
      --db /seed/skemat.db \
      --schema internal/store/schema.sql

# ---- runtime ----
FROM alpine:3.20
RUN adduser -D -u 10001 app
COPY --from=build /out/skemat-server /usr/local/bin/skemat-server
COPY --from=seed  /seed/live     /data/live
COPY --from=seed  /seed/skemat.db /data/skemat.db
RUN chown -R app:app /data
USER app
ENV SKEMAT_ADDR=:8080 \
    SKEMAT_DATA=/data/live \
    SKEMAT_DB=/data/skemat.db \
    SKEMAT_ADMIN_EMAIL=admin@example.com
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/skemat-server"]