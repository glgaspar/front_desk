# Front Desk API

Front Desk is a powerful, self-hosted backend built in Go that acts as a central management system for your home server or cloud environment. It provides a RESTful API to manage Docker containers, orchestrate builds, stream real-time logs, and integrate with various homelab services like Cloudflare Tunnels, Pi-hole, and Transmission.

## Features

*   **App & Container Management**: Create, start, stop, and remove Docker containers. Dynamically rebuild apps, edit `docker-compose.yml` configurations, and fetch container state/logs.
*   **Real-time Streaming**: Uses Apache Kafka to stream real-time Docker build and execution logs via Server-Sent Events (SSE) directly to the frontend.
*   **Authentication & Access Control**: Built-in user authentication and secure cookie-based session management backed by PostgreSQL. Includes a "first access" safeguard to register the initial admin user.
*   **Service Integrations**:
    *   **Cloudflare**: Automate the creation of Cloudflare Zero Trust tunnels and DNS routing for your self-hosted apps.
    *   **Pi-hole**: Configure Pi-hole and retrieve DNS query history.
    *   **Transmission**: Manage torrents (start/stop) and update configurations.
*   **System Dashboard**: Fetch host system usage statistics and manage customizable dashboard widgets.

## Tech Stack

*   **Language:** Go
*   **Web Framework:** Echo v4
*   **Message Broker:** Apache Kafka & Zookeeper (via `segmentio/kafka-go`)
*   **Database:** PostgreSQL
*   **Containerization:** Docker & Docker Compose

## Prerequisites

Before you begin, ensure you have the following installed on your host machine:

*   Docker and Docker Compose
*   Go 1.20+ (if running locally outside of Docker)
*   A running instance of PostgreSQL

## Getting Started

### 1. Clone the repository

```bash
git clone <repository-url>
cd front_desk
```

### 2. Configure Environment Variables

Create a `.env` file in the root directory. You will need to configure variables such as:

```env
ENVIROMENT=DEV
FRONT_END_URL=http://localhost:3000
DOMAIN_NAME=localhost
FIRST_ACCESS=YES
# Database configurations (used by connection package)
PG_HOST=...
PG_PORT=5432
PG_USER=...
PG_PASSWORD=...
PG_DBNAME=...
# where your apps live. use the same path inside and outside the container
APPS_DIR=/mnt/apps
# Kafka Configurations
KAFKA_IP=kafka
KAFKA_PORT=29092
```

### 3. Running with Docker Compose

The provided `docker-compose.yml` includes the API, Kafka, and Zookeeper. The API container mounts the host's Docker socket (`/var/run/docker.sock`) to manage containers.

```bash
docker compose up -d --build
```

The apps folder is mounted with the **same path** on the host and inside the container (`/mnt/apps:/mnt/apps`). Docker compose resolves relative volumes (`./data`) from the project folder and the docker daemon reads that path on the host, so if the paths are different your app volumes end up in the wrong place. If you change it, change `APPS_DIR` too.

## App labels

Front Desk only cares about containers with these labels:

```yaml
labels:
  front-desk.name: Jellyfin
  front-desk.url: https://jellyfin.yourdomain.com
  front-desk.dir: /jellyfin
  front-desk.logo: https://example.com/logo.png
  front-desk.port: 8096   # used by cloudflare and nginx proxy manager (front-desk.cloudflare-port still works)
```

## Key API Endpoints

### Authentication
*   `POST /register` - Register the initial user (Requires `FIRST_ACCESS=YES` in env).
*   `POST /login` - Authenticate and receive a session cookie.
*   `GET /login/logout` - Destroy the current session.

### Applications (Docker Management)
*   `GET /apps` - List all installed applications/containers.
*   `POST /apps/create` - Build and spin up a new application via Docker Compose.
*   `PUT /apps/toggleOnOff/:id/:toggle` - Start or stop a specific container.
*   `GET /apps/logs/:id` - Stream live container logs.
*   `GET /apps/waitingBuilds/:app` - Listen to live build logs via SSE.
*   `PUT /apps/restart/:id` - Restart a container.
*   `POST /apps/update/:id` - Pull new images and recreate the app (follow it like a build).
*   `GET /apps/updates` - Check every app for a newer image (`?force=true` skips the cache).
*   `GET /apps/stats` - CPU and memory per container.

### System Monitoring
*   `GET /system/usage` - Fetch CPU, memory, and top process usage statistics from the host machine.
*   `GET /monitoring/uptime?range=24h` - Uptime per app (`1h`, `24h`, `7d`, `30d`).
*   `GET /monitoring/uptime/history?app=<key>&range=` - Uptime and latency over time for one app.
*   `GET /monitoring/metrics?range=` - Host CPU, memory and disk over time.
*   `GET|POST /notifications`, `PUT|DELETE /notifications/:id`, `POST /notifications/:id/test` - Where to yell when something goes down (ntfy, Discord, Telegram or a webhook).

### Integrations
*   `GET /cloudflare/config` / `POST /cloudflare/config` - Manage Cloudflare tunnels.
*   `GET /cloudflare/routes`, `GET /cloudflare/dns` - What is going through the tunnel.
*   `POST /cloudflare/hostname`, `DELETE /cloudflare/hostname/:hostname?dns=true` - Add or remove a public hostname.
*   `GET /npm/config` / `POST /npm/config` - Connect to Nginx Proxy Manager (url, login, the host it should forward to and a default certificate).
*   `GET /npm/hosts`, `POST /npm/hosts`, `DELETE /npm/hosts/:id` - Proxy hosts for your apps.
*   `GET /npm/certificates`, `PUT /npm/hosts/:id/certificate` - Pick which existing certificate goes on each host. Front Desk does not request certificates, do that on NPM.
*   `GET /pihole/history` - Retrieve Pi-hole query history.
*   `GET /transmission/torrents` - List active torrents from Transmission.

### Widget Control for the FrontEnd
*   `GET /widgets` - Retrieve dashboard widgets.
*   `POST /widgets` - Create a new dashboard widget.
*   `PUT /widgets/toggle/:id/:toggle` - Enable or disable a specific widget.
