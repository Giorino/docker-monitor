# Use an official Golang runtime as a parent image
FROM golang:1.23-alpine AS builder

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

# Copy the source from the current directory to the Working Directory inside the container
COPY . .

# Build the Go app
RUN go build -o docker-monitor

# Start a new stage from scratch
FROM alpine:latest as production

# Set the Current Working Directory inside the container
WORKDIR /root/

# Copy the Pre-built binary file from the previous stage
COPY --from=builder /app/docker-monitor .

# Expose port 8080 to the outside world
EXPOSE 8080

# Command to run the executable
CMD ["./docker-monitor"]