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

FROM quay.io/jupyter/base-notebook:latest

WORKDIR /app

# Set permissions
USER root

RUN apt-get update && apt-get install -y \
    uuid-runtime \
    gcc \
    wget \
    pkg-config \
    libcairo2-dev \
    python3-dev \
    libc6-dev \
    binutils \
    libdbus-1-dev \
    gawk \
    autoconf \
    net-tools \
    gcc make build-essential

# Set the Python version you want to install
ARG PYTHON_VERSION=3.12.1

# Download and extract Python source
RUN wget https://www.python.org/ftp/python/${PYTHON_VERSION}/Python-${PYTHON_VERSION}.tgz && \
    tar -xzf Python-${PYTHON_VERSION}.tgz

# Build and install Python
WORKDIR /app/Python-${PYTHON_VERSION}
RUN ./configure --enable-optimizations && \
    make -j "$(nproc)" && \
    make altinstall

# cleanup
RUN rm -rf /app/Python-${PYTHON_VERSION} && \
    rm /app/Python-${PYTHON_VERSION}.tgz

# Download and install Miniconda
RUN wget https://repo.anaconda.com/miniconda/Miniconda3-latest-Linux-x86_64.sh -O miniconda.sh \
    && bash miniconda.sh -b -p /app/miniconda \
    && rm miniconda.sh

# Add Conda to your PATH
ENV PATH="/app/miniconda/bin:${PATH}"

# # Create conda environment
COPY codeexecjupyter.yml /app/codeexecjupyter.yml
RUN conda env create --prefix /app/condaapp --file /app/codeexecjupyter.yml --force

# # Make RUN commands use the conda environment:
SHELL ["conda", "run", "-p", "/app/condaapp", "/bin/bash", "-c"]

# copy the requirements file
COPY requirements.txt /app/requirements.txt

# Install Python packages
RUN python -m ensurepip
RUN pip install -r /app/requirements.txt

# Create a directory for the data
RUN mkdir -p /mnt/data && chmod 777 /mnt/data

WORKDIR /app

# Change owner and group of the copied files
COPY --chown=jovyan:jovyan --from=builder /app/goclientapp .
COPY --chown=jovyan:jovyan --from=builder /app/entrypoint.sh .

RUN usermod -aG sudo jovyan && echo 'jovyan ALL=(ALL) NOPASSWD:ALL' >> /etc/sudoers

# Switch back to the jovyan user
USER jovyan

# Ensure the script is executable
RUN chmod +x /app/goclientapp
RUN chmod +x /app/entrypoint.sh

# Use the "exec" form of CMD to ensure that the server
# becomes PID 1, and thus receives Unix signal notifications,
# and that a signal proxy isn't spawned
ENTRYPOINT ["/app/entrypoint.sh"]