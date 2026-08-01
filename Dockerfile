# Build stage
FROM golang:1.25-alpine AS builder

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown

LABEL maintainer="Thanawy Team"
LABEL description="Thanawy Backend API"
LABEL version="1.0"

RUN apk add --no-cache git
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY pkg/ ./pkg/
COPY docs/ ./docs/
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w -X thanawy-backend/pkg/buildinfo.Version=${VERSION} -X thanawy-backend/pkg/buildinfo.Commit=${COMMIT} -X thanawy-backend/pkg/buildinfo.BuildTime=${BUILD_TIME}" -o main ./cmd/api/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o migrate ./cmd/migrate/main.go

FROM alpine:3.20
RUN apk add --no-cache ca-certificates curl

LABEL maintainer="Thanawy Team"
LABEL description="Thanawy Backend API Runtime"
LABEL version="1.0"
WORKDIR /app
COPY --from=builder /app/main .
COPY --from=builder /app/migrate .
RUN adduser -D nonroot && mkdir -p /app/uploads && chown -R nonroot:nonroot /app
USER nonroot
EXPOSE 8082 50051
HEALTHCHECK --interval=30s --timeout=5s --start-period=60s --retries=3 \
  CMD curl --silent --fail http://localhost:8082/health || exit 1
CMD ["./main"]
