ALTER TABLE events DROP COLUMN IF EXISTS event_schema_version;
DROP TABLE IF EXISTS event_type_versions;
ALTER TABLE event_types DROP COLUMN IF EXISTS current_version;
