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
RUN go build -o goclientapp .

WORKDIR /app

RUN apt-get update && apt-get install -y \
    pkg-config \
    libcairo2-dev \
    python3-dev \
    gcc \
    libc6-dev \
    linux-headers-amd64 \
    binutils \
    libdbus-1-dev \
    gawk \
    autoconf \
    net-tools

FROM jupyter/base-notebook:latest

WORKDIR /app

# Set permissions
USER root

# Change owner and group of the copied files
COPY --chown=jovyan:jovyan --from=builder /app/goclientapp .
COPY --chown=jovyan:jovyan --from=builder /app/entrypoint.sh .

RUN usermod -aG sudo jovyan && echo 'jovyan ALL=(ALL) NOPASSWD:ALL' >> /etc/sudoers

# Switch back to the jovyan user
USER jovyan

RUN chmod +x /app/goclientapp
#COPY requirements.txt /app/requirements.txt

# Install Python packages
# RUN python -m ensurepip
# RUN pip install -r /app/requirements.txt

# Ensure the script is executable
RUN chmod +x /app/entrypoint.sh
#RUN mkdir -p /mnt/data && chmod 777 /mnt/data

# Use the "exec" form of CMD to ensure that the server
# becomes PID 1, and thus receives Unix signal notifications,
# and that a signal proxy isn't spawned
ENTRYPOINT ["/app/entrypoint.sh"]