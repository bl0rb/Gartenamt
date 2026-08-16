# Modules Overview

This repository contains separate modules for distributed deployment.

## Modules

1. `public-web`
- Public website module
- Static pages served by Nginx

2. Root app
- Internal Verwaltung App
- Business logic, admin interface, backup/import tools

## Run All Modules

From repository root:

```bash
docker compose -f docker-compose.modular.yml up --build
```

## Network Separation

- `internal` network: Verwaltung App
- `public` network: Public Webpage
