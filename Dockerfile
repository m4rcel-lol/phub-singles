# syntax=docker/dockerfile:1

# ---------------------------------------------------------------------------
# Stage 1 — build the Angular bundle.
# ---------------------------------------------------------------------------
FROM node:24-alpine AS frontend

WORKDIR /app

# Dependencies first so the layer is reused whenever only sources change.
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci --no-audit --no-fund

COPY frontend/ ./
RUN npm run build


# ---------------------------------------------------------------------------
# Stage 2 — compile the Go binary with the frontend embedded in it.
# ---------------------------------------------------------------------------
FROM golang:1.26-alpine AS backend

WORKDIR /src

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./
# go:embed picks the bundle up from internal/web/dist.
COPY --from=frontend /app/dist/frontend/browser ./internal/web/dist

# CGO is off: modernc.org/sqlite is pure Go, so the result is a static binary.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/phs-server ./cmd/server

# Pre-create the data directory with the runtime uid so a fresh named volume
# inherits the right ownership.
RUN mkdir -p /out/data/uploads


# ---------------------------------------------------------------------------
# Stage 3 — runtime: distroless, non-root, ~20 MB total.
# ---------------------------------------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=backend /out/phs-server /usr/local/bin/phs-server
COPY --from=backend --chown=65532:65532 /out/data /data

USER 65532:65532
WORKDIR /data
EXPOSE 8080

ENV PHS_ADDR=:8080 \
    PHS_DB_PATH=/data/phs.db \
    PHS_UPLOAD_DIR=/data/uploads

# The binary probes itself; the image has no shell or wget.
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD ["/usr/local/bin/phs-server", "-healthcheck"]

ENTRYPOINT ["/usr/local/bin/phs-server"]
