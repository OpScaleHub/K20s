# Builder stage
FROM docker.io/library/golang:1.25 AS builder

# Set the working directory
WORKDIR /workspace

# Copy go.mod and go.sum to download dependencies
COPY go.mod go.sum ./

# Download dependencies. This is cached if go.mod and go.sum don't change.
RUN go mod download

# Copy the source code.
# By copying directories separately, we leverage Docker's cache.
# A change in one directory will not invalidate the cache for the others.
COPY api/ api/
COPY cmd/ cmd/
COPY internal/ internal/

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o /main ./cmd/main.go

# Certificates stage
FROM alpine:3.21.3 AS certs
RUN apk add --no-cache ca-certificates

# Final stage
FROM scratch
ENV PATH=/bin
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /main /main
EXPOSE 8080
ENTRYPOINT ["/main"]