#!/bin/sh
set -eu

if [ -z "${DATABASE_URL:-}" ]; then
  echo "DATABASE_URL is required" >&2
  exit 1
fi

backup_dir=${MOTIF_BACKUP_DIR:-./backups}
mkdir -p "$backup_dir"
timestamp=$(date -u +%Y%m%dT%H%M%SZ)
output="$backup_dir/motif-$timestamp.dump"
pg_dump --format=custom --no-owner --no-privileges "$DATABASE_URL" > "$output"
chmod 600 "$output"
printf '%s\n' "$output"
