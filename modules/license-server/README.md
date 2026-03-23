# License Server

Standalone Go service for license lifecycle management.

## Endpoints

- `GET /health`
- `POST /v1/licenses/issue` (admin token + private key required)
- `POST /v1/licenses/validate` (client token optional)
- `POST /v1/licenses/revoke` (admin token + private key required)

## Environment Variables

Required:
- none for local development; keys are generated automatically on first start

Optional:
- `LICENSE_PRIVATE_KEY_BASE64` (use a fixed private key instead of an auto-generated one)
- `LICENSE_PUBLIC_KEY` (matching public key for a fixed private key)
- `LICENSE_SERVER_ADMIN_TOKEN` (overrides the auto-generated persistent admin token)
- `LICENSE_SERVER_CLIENT_TOKEN` (required only if `/v1/licenses/validate` should enforce a bearer token)
- `LICENSE_DB_PATH` (default: `./licenses.db`)
- `LICENSE_SERVER_ADDR` (default: `:8090`)

## Local Run

```bash
cd modules/license-server
go mod tidy
go run .
```

The server generates its keypair automatically on first start.
The server also generates and persists an admin token automatically, and the web UI fills it in for license creation.

## Docker Build

```bash
docker build -t kleingarten-license-server ./modules/license-server
```
