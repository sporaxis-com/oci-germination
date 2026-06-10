-- ck-allinone first-boot SQL — piped through `postgres --single` by
-- ociger-pg-launcher immediately after initdb, before the supervised
-- postgres process starts. Idempotent in spirit (IF NOT EXISTS), but
-- the surrounding gate (PG_VERSION absent → first boot only) is the
-- real guarantee that this never runs twice on a persistent volume.
--
-- v3.9 Critical Isolation Alpha (v0.7.8+): CREATE EXTENSION pgck CASCADE
-- runs pgCK v0.3.3's bootstrap, which CREATEs the ck_substrate /
-- ck_participant roles and applies the REVOKE FROM PUBLIC floor + the
-- SECURITY DEFINER grant of ckp.dispatch to ck_participant. The launcher
-- appends `ALTER ROLE ck_participant WITH LOGIN PASSWORD '<env>'` to this
-- SQL stream when OCIGER_CK_PARTICIPANT_PASSWORD is set, so the role
-- becomes a usable login under the floor for external consumers (web2/,
-- etc.). If unset, the role exists but cannot log in.
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS pgrdf;
CREATE EXTENSION IF NOT EXISTS pgck CASCADE;
