# Modules Overview

This repository contains separate modules for distributed deployment.

## Modules

1. `license-server`
- Standalone license authority service
- Handles issue, validate, revoke endpoints

2. `public-web`
- Public website module
- Static pages served by Nginx

3. Root app
- Internal Verwaltung App
- Business logic, admin interface, backup/import tools

## Run All Modules

From repository root:

```bash
docker compose -f docker-compose.modular.yml up --build
```

## Network Separation

- `internal` network: Verwaltung App + License Server
- `public` network: Public Webpage
