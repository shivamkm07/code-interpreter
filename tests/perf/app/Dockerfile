# Use the official Golang image as the builder stage
FROM golang:1.21-bullseye  AS builder

# Set the working directory inside the builder stage
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

# Copy the source code from the current directory to the working directory inside the builder stage
COPY . .

# Build the Go application inside the builder stage
RUN go build -o app .

# Use a lightweight base image for the final stage
FROM debian:bullseye-slim

# Set the working directory inside the final stage
WORKDIR /app

RUN apt-get update && apt-get install -y ca-certificates

# Copy the built executable from the builder stage to the final stage
COPY --from=builder /app/app .

# Set the entry point for the container
CMD ["./app"]

