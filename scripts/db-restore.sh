#!/bin/sh
set -eu

if [ -z "${DATABASE_URL:-}" ]; then
  echo "DATABASE_URL is required" >&2
  exit 1
fi
if [ "$#" -ne 1 ]; then
  echo "usage: scripts/db-restore.sh BACKUP_FILE" >&2
  exit 1
fi
if [ "${MOTIF_CONFIRM_RESTORE:-}" != "yes" ]; then
  echo "Set MOTIF_CONFIRM_RESTORE=yes to restore; this replaces database contents." >&2
  exit 1
fi
pg_restore --clean --if-exists --no-owner --no-privileges --dbname "$DATABASE_URL" "$1"
