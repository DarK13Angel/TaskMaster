# =====================================================================
# TaskMaster Multi-Stage Dockerfile
# Produces a self-contained, lightweight (~25MB) container with embedded UI
# =====================================================================

# ---------------------------------------------------------------------
# Stage 1: Build Vue 3 Frontend Assets
# ---------------------------------------------------------------------
FROM node:22-alpine AS frontend-builder
WORKDIR /app/web

# Install frontend dependencies
COPY web/package*.json ./
RUN npm ci

# Build static assets into /app/web/dist
COPY web/ ./
RUN npm run build

# ---------------------------------------------------------------------
# Stage 2: Build Go Backend with Embedded Frontend
# ---------------------------------------------------------------------
FROM golang:alpine AS backend-builder
WORKDIR /app

# Install git for dependency resolution if needed
RUN apk add --no-cache git

# Cache Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy source code and frontend build output
COPY . .
COPY --from=frontend-builder /app/web/dist ./web/dist

# Build pure Go static binary (CGO disabled for maximum portability)
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w -extldflags '-static'" -o taskmaster ./cmd/taskmaster

# ---------------------------------------------------------------------
# Stage 3: Minimal Production Runtime Container
# ---------------------------------------------------------------------
FROM alpine:3.21

# Install CA certificates and timezone data
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy compiled standalone binary
COPY --from=backend-builder /app/taskmaster /app/taskmaster

# Default runtime configuration
ENV PORT=8088 \
    TASK_TTL_DAYS=30 \
    CLEANUP_INTERVAL_HOURS=6 \
    TASKMASTER_DATA_DIR=/data

# Create persistent storage directory for SQLite database
RUN mkdir -p /data
VOLUME ["/data"]

EXPOSE 8088

# Health check to ensure service responsiveness
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:${PORT:-8088}/api/v1/health || exit 1

ENTRYPOINT ["/app/taskmaster"]
