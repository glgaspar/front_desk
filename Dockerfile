FROM golang:1.23.1

WORKDIR /src

# Install Docker CLI and Compose plugin
RUN apt-get update && apt-get install -y \
    docker.io \
    curl \
    jq \
    unzip && \
    # Install docker-compose plugin (v2)
    mkdir -p ~/.docker/cli-plugins && \
    curl -SL https://github.com/docker/compose/releases/download/v2.27.0/docker-compose-linux-x86_64 -o ~/.docker/cli-plugins/docker-compose && \
    chmod +x ~/.docker/cli-plugins/docker-compose && \
    ln -s ~/.docker/cli-plugins/docker-compose /usr/local/bin/docker-compose && \
    ln -s ~/.docker/cli-plugins/docker-compose /usr/local/bin/docker compose && \
    rm -rf /var/lib/apt/lists/*

# Copy Go module files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the entire project
COPY . .

# Build Go binary
RUN go build -o /front_desk

# Default command
CMD ["/front_desk"]
