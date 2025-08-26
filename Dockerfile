FROM golang:1.23.1

WORKDIR /app

# Install Docker CLI
RUN apt-get update && apt-get install -y docker.io && rm -rf /var/lib/apt/lists/*

# Copy Go module files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the entire project
COPY . .

# Build Go binary
RUN go build -o /front_desk

# Default command
CMD ["/front_desk"]
