# syntax=docker/dockerfile:1

# ---- build stage ----
# Pure-Go build (modernc sqlite, no CGO) yields a static binary that runs on a
# minimal base image.
FROM golang:1.24-alpine AS build

WORKDIR /src

# Cache module downloads separately from source for faster rebuilds.
COPY backend/go.mod backend/go.sum ./backend/
WORKDIR /src/backend
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY backend/ /src/backend/
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /out/keirouter ./cmd/keirouter

# ---- runtime stage ----
FROM gcr.io/distroless/static-debian12:nonroot

# Persisted data (db, master key) lives here; mount a volume to keep it.
ENV KEIROUTER_DATA__DIR=/data
# Listen on all interfaces inside the container; restrict exposure at the edge.
ENV KEIROUTER_SERVER__HOST=0.0.0.0
ENV KEIROUTER_SERVER__PORT=20180
# The admin API is loopback-only by default, which blocks container access.
# Disable only behind a trusted reverse proxy / network policy.
ENV KEIROUTER_SECURITY__BIND_LOOPBACK_ONLY=false

COPY --from=build /out/keirouter /usr/local/bin/keirouter

USER nonroot:nonroot
EXPOSE 20180
VOLUME ["/data"]

ENTRYPOINT ["/usr/local/bin/keirouter"]