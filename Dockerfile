# Build Stage
FROM golang:1.18 AS builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

# Copy the source from the current directory to the Working Directory inside the container
COPY . .

# Build the Go app
RUN go build -o proxyapp .

# Run Stage
FROM cappsinttestregistryprivate.azurecr.io/codeexecjupyter:latest

WORKDIR /app

# Copy the binary and entrypoint.sh from the builder stage
COPY --from=builder /app/proxyapp /app/proxyapp

RUN chmod 777 /app/proxyapp
COPY sessions_entrypoint.sh /app/sessions_entrypoint.sh

# Ensure the script is executable
RUN chmod +x /app/sessions_entrypoint.sh
RUN mkdir -p /mnt/data && chmod -R 777 /mnt/data 
# Use the "exec" form of CMD to ensure that the server
# becomes PID 1, and thus receives Unix signal notifications,
# and that a signal proxy isn't spawned
ENTRYPOINT ["/app/sessions_entrypoint.sh"]
