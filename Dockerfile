# Use the official Golang image as base
FROM golang:1.23.1

# Set working directory
WORKDIR /src

# Install Docker CLI, Compose plugin, and other dependencies
RUN apt-get update && \
    apt-get install -y docker.io curl jq unzip && \
    # Install Docker Compose V2 plugin
    mkdir -p /usr/local/lib/docker/cli-plugins && \
    curl -SL https://github.com/docker/compose/releases/download/v2.27.0/docker-compose-linux-x86_64 -o /usr/local/lib/docker/cli-plugins/docker-compose && \
    chmod +x /usr/local/lib/docker/cli-plugins/docker-compose && \
    ln -s /usr/local/lib/docker/cli-plugins/docker-compose /usr/local/bin/docker-compose && \
    rm -rf /var/lib/apt/lists/*

# Copy Go module files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the entire project source
COPY . .

# Build the Go binary
RUN go build -o /front_desk

# Default command when container starts
CMD ["/front_desk"]
