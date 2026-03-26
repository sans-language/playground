# Sans Playground Server

API server for the [Sans language](https://sans.dev) web playground. Compiles and runs Sans code in sandboxed Docker containers.

## API Endpoints

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/api/run` | POST | Compile + run Sans code, return stdout/stderr |
| `/api/share` | POST | Save code snippet, return short ID |
| `/api/snippet/:id` | GET | Retrieve saved snippet by ID |
| `/api/health` | GET | Health check |

## Development

Requires: Go 1.22+, Docker

```sh
go build -o playground-server .
./playground-server -addr :8090 -db playground.db
```

## Deployment

See `deploy/` for Nginx config, systemd service, and setup script.

## Sync with Sans releases

This repo's CI listens for `repository_dispatch` events from `sans-language/sans`. On each Sans release, the Docker sandbox image is rebuilt with the latest compiler binary.
