# Armonite Distributed Load Testing Framework
# Multi-stage Docker build for coordinator and agents

# Stage 1: Build React UI
FROM node:18-alpine AS ui-builder
WORKDIR /app/ui
COPY ui-react/package*.json ./
RUN npm ci --only=production
COPY ui-react/ ./
RUN npm run build

# Stage 2: Build Go binary
FROM golang:1.21-alpine AS go-builder
WORKDIR /app
RUN apk add --no-cache git ca-certificates tzdata
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=ui-builder /app/ui-build ./ui-build
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static" -X main.version='${VERSION} -o armonite .

# Stage 3: Final runtime image
FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY --from=go-builder /app/armonite .
COPY --from=go-builder /app/ui-build ./ui-build
COPY --from=go-builder /app/armonite.yaml ./armonite.yaml

# Create non-root user
RUN adduser -D -s /bin/sh armonite
USER armonite

# Expose ports
EXPOSE 4222 8080 8081

# Default command
CMD ["./armonite", "coordinator", "--ui"]