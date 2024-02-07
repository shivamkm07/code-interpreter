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
RUN go build -o acamanager .

FROM cappsinttestregistryprivate.azurecr.io/codeexecjupyter:latest

WORKDIR /app

COPY --from=builder /app/acamanager /app/acamanager

RUN tdnf makecache && tdnf install -y \
    pkg-config \
    cairo-devel \
    python3-devel \
    gcc \
    glibc-devel \
    kernel-headers \
    binutils \
    dbus-devel \
    awk \
    autoconf

RUN chmod +x /app/acamanager
COPY sessions_entrypoint.sh /app/sessions_entrypoint.sh
COPY requirements.txt /app/requirements.txt

# Install Python packages
RUN python -m ensurepip
RUN pip install -r /app/requirements.txt

# Ensure the script is executable
RUN chmod +x /app/sessions_entrypoint.sh
RUN mkdir -p /mnt/data && chmod 777 /mnt/data

# Use the "exec" form of CMD to ensure that the server
# becomes PID 1, and thus receives Unix signal notifications,
# and that a signal proxy isn't spawned
ENTRYPOINT ["/app/sessions_entrypoint.sh"]

LABEL TlsEnabled=false
