-- ck-allinone first-boot SQL — piped through `postgres --single` by
-- ociger-pg-launcher immediately after initdb, before the supervised
-- postgres process starts. Idempotent in spirit (IF NOT EXISTS), but
-- the surrounding gate (PG_VERSION absent → first boot only) is the
-- real guarantee that this never runs twice on a persistent volume.
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS pgrdf;
CREATE EXTENSION IF NOT EXISTS pgck CASCADE;
