#!/bin/sh
set -eu

it_suffix="$(date +%s)-$$"
it_postgres_container="motif-it-postgres-$it_suffix"
it_nats_container="motif-it-nats-$it_suffix"

cleanup() {
  docker stop "$it_nats_container" "$it_postgres_container" >/dev/null 2>&1 || true
}
trap cleanup EXIT HUP INT TERM

docker run --rm -d --name "$it_postgres_container" \
  -e POSTGRES_USER=motif \
  -e POSTGRES_PASSWORD=motif_local_only \
  -e POSTGRES_DB=motif \
  -p 127.0.0.1::5432 postgres:16-alpine >/dev/null
docker run --rm -d --name "$it_nats_container" \
  -p 127.0.0.1::4222 nats:2.10-alpine -js >/dev/null

it_ready=false
for it_attempt in $(seq 1 80); do
  if docker exec "$it_postgres_container" pg_isready -U motif -d motif >/dev/null 2>&1 && \
    docker logs "$it_nats_container" 2>&1 | grep -q 'Server is ready'; then
    it_ready=true
    break
  fi
  sleep 0.25
done
if [ "$it_ready" != true ]; then
  echo "Disposable PostgreSQL or NATS did not become ready" >&2
  exit 1
fi

it_postgres_port="$(docker port "$it_postgres_container" 5432/tcp)"
it_postgres_port="${it_postgres_port##*:}"
it_nats_port="$(docker port "$it_nats_container" 4222/tcp)"
it_nats_port="${it_nats_port##*:}"

MOTIF_INTEGRATION_DATABASE_URL="postgres://motif:motif_local_only@127.0.0.1:$it_postgres_port/motif?sslmode=disable" \
MOTIF_INTEGRATION_NATS_URL="nats://127.0.0.1:$it_nats_port" \
  go test -tags integration ./tests/integration -v "$@"
