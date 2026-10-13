#!/bin/sh
set -eu

api_url=${MOTIF_API_URL:-http://127.0.0.1:18080}
worker_url=${MOTIF_WORKER_URL:-http://127.0.0.1:18081}
curl --fail --silent --show-error "$api_url/health/ready" >/dev/null
curl --fail --silent --show-error "$worker_url/health/ready" >/dev/null
if [ "${MOTIF_CHECK_METRICS:-true}" = "true" ]; then
  curl --fail --silent --show-error "$api_url/metrics" >/dev/null
fi
printf '%s\n' "MotifEngine API, worker, and metrics are reachable."
