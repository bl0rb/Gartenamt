Place TLS certificate files for public web HTTPS here.

Required files:
- fullchain.pem
- privkey.pem

These files are mounted read-only into the container at:
/etc/nginx/certs

For NAS production, provide valid certificates from your CA or reverse proxy.
