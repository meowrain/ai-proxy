# ---- Build Stage ----
FROM golang:1.23-alpine AS builder

# Define proxy arguments
ARG HTTP_PROXY
ARG HTTPS_PROXY
ARG SOCKS_PROXY
ARG NO_PROXY

# Set them as environment variables for the build stage
ENV http_proxy=$HTTP_PROXY
ENV https_proxy=$HTTPS_PROXY
ENV socks_proxy=$SOCKS_PROXY
ENV no_proxy=$NO_PROXY

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

# Copy the source code into the container
COPY . .

# Build the Go app
# -ldflags \"-s -w\" strips debug information and symbols, reducing binary size
# CGO_ENABLED=0 builds a statically linked binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags \"-s -w\" -o ai-proxy .

# ---- Run Stage ----
FROM alpine:latest

WORKDIR /app

# Copy the Pre-built binary file from the previous stage
COPY --from=builder /app/ai-proxy .

# Copy the configuration file
# Ensure api.json is in the same directory as the Dockerfile when building,
# or adjust the COPY path accordingly.
COPY api.json .

# Expose port (e.g., 8090, or the port configured in your api.json)
# You might want to make this an ARG or ENV if it's highly configurable
EXPOSE 8090

# Command to run the executable
CMD [\"./ai-proxy\"]
