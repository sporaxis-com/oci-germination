-- ck-allinone first-boot SQL — piped through `postgres --single` by
-- ociger-pg-launcher immediately after initdb, before the supervised
-- postgres process starts. Idempotent in spirit (IF NOT EXISTS), but
-- the surrounding gate (PG_VERSION absent → first boot only) is the
-- real guarantee that this never runs twice on a persistent volume.
--
-- pgCK v0.4.2 "install-from-zero": CREATE EXTENSION pgck CASCADE is
-- genuinely sufficient. The durable tables (instances/ledger/proof/
-- outbox) are top-level install DDL born owned by ck_substrate; the
-- CI-A role floor covers every ckp function; ontology modules ship at
-- /ontology with the extension artifact. The v0.7.14..v0.7.16 era
-- mitigations (CALL ckp.bootstrap_kernel, ALTER TABLE OWNER, pgrdf
-- grants) are retired — the pgrdf grants in particular BREACHED the
-- v3.9 floor (ck_participant must reach nothing but ckp.dispatch) and
-- pgCK's install-cascade RESPONSE asked for their removal.
--
-- The launcher appends `ALTER ROLE ck_participant WITH LOGIN PASSWORD
-- '<env>'` to this SQL stream when OCIGER_CK_PARTICIPANT_PASSWORD is
-- set. If unset, the role exists but cannot log in.
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS pgrdf;
CREATE EXTENSION IF NOT EXISTS pgck CASCADE;

-- Arm the default 'demo' project board from the artifact-shipped ontology
-- modules so the bundle dispatches out of the box (pgCK 0.4.2 §4: optional
-- board arming; /ontology is the documented default root).
CALL ckp.import_module('task', 'demo', '/ontology');
CALL ckp.import_module('goal', 'demo', '/ontology');

-- Native outbox drain role (v0.7.18+). Every seal enqueues an event into
-- ckp.outbox; pgCK's in-kernel nats-client bgworker that drains it onto NATS
-- is not in the shipped .so, so ociger-pgck-relay drains it (the same shim
-- posture as the dispatch bridge — it retires when pgCK ships the bgworker).
-- The drain runs as this dedicated LEAST-PRIVILEGE role: SELECT + DELETE on
-- ckp.outbox only. It is deliberately NOT ck_participant (the v3.9 floor keeps
-- the participant role to ckp.dispatch alone) and NOT a superuser. ck_drainer
-- connects from inside the container over 127.0.0.1 (the launcher's pg_hba
-- catch-all trusts local connections); it needs no password and is not
-- exposed — only ck_participant is the externally-scram'd role.
DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'ck_drainer') THEN
    CREATE ROLE ck_drainer LOGIN;
  END IF;
END $$;
GRANT USAGE ON SCHEMA ckp TO ck_drainer;
GRANT SELECT, DELETE ON ckp.outbox TO ck_drainer;
