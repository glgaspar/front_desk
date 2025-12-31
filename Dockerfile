# Use the official Golang image as base
FROM golang:1.23.1

# Set working directory
WORKDIR /src

# Install Docker CLI, Compose plugin, and other dependencies
# Install dependencies and the official Docker CLI
RUN apt-get update && \
    apt-get install -y ca-certificates curl gnupg && \
    install -m 0755 -d /etc/apt/keyrings && \
    curl -fsSL https://download.docker.com/linux/debian/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg && \
    chmod a+r /etc/apt/keyrings/docker.gpg && \
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/debian bookworm stable" | \
    tee /etc/apt/sources.list.d/docker.list > /dev/null && \
    apt-get update && \
    apt-get install -y docker-ce-cli docker-compose-plugin jq unzip && \
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
