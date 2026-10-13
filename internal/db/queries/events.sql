-- name: GetEventByExternalID :one
SELECT id, tenant_id, environment_id, external_event_id, event_type, status
FROM events
WHERE tenant_id = $1 AND environment_id = $2 AND external_event_id = $3;

-- name: ListPendingOutbox :many
SELECT event_id, attempts
FROM event_outbox
WHERE published_at IS NULL
ORDER BY created_at;
