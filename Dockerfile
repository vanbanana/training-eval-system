# Stage 1: Build backend
FROM golang:1.25-alpine AS builder
WORKDIR /build
COPY go-backend/go.mod go-backend/go.sum ./
RUN go mod download
COPY go-backend/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/server ./cmd/server

# Stage 2: Build frontend
FROM node:22-alpine AS frontend-builder
WORKDIR /build
COPY frontend/package.json frontend/pnpm-lock.yaml ./
RUN corepack enable && pnpm install --frozen-lockfile
COPY frontend/ ./
RUN pnpm build

# Stage 3: Runtime
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/server .
COPY --from=frontend-builder /build/dist ./dist
EXPOSE 8080
ENTRYPOINT ["./server"]