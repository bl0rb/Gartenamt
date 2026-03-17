# License Server

Standalone Go service for license lifecycle management.

## Endpoints

- `GET /health`
- `POST /v1/licenses/issue` (admin token + private key required)
- `POST /v1/licenses/validate` (client token optional)
- `POST /v1/licenses/revoke` (admin token + private key required)

## Environment Variables

Required:
- `LICENSE_PUBLIC_KEY`

Optional:
- `LICENSE_PRIVATE_KEY_BASE64` (enables issue/revoke)
- `LICENSE_SERVER_ADMIN_TOKEN`
- `LICENSE_SERVER_CLIENT_TOKEN`
- `LICENSE_DB_PATH` (default: `/data/licenses.db`)
- `LICENSE_SERVER_ADDR` (default: `:8090`)

## Local Run

```bash
cd modules/license-server
go mod tidy
go run .
```

## Docker Build

```bash
docker build -t kleingarten-license-server ./modules/license-server
```
