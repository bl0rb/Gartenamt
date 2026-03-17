# Public Web Module

Static public website module served by Nginx.

## Contents

- `site/`: public HTML pages
- `nginx.conf`: web server routing
- `Dockerfile`: container image build

## Local Docker Build

```bash
docker build -t kleingarten-public-web ./modules/public-web
```

## Run

```bash
docker run --rm -p 8081:80 kleingarten-public-web
```
