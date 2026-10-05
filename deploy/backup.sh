#!/bin/sh
set -eu
cd "$(dirname "$0")"
mkdir -p backups
backup="backups/homebox-custom-$(date +%Y%m%d-%H%M%S).tar.gz"
docker compose stop
trap 'docker compose start' EXIT INT TERM
# Database, WAL, uploads and private application configuration are captured together.
tar -czf "$backup" data .env compose.yaml
printf 'Backup written: %s\n' "$backup"
