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

-- Arm the kernel from the artifact-shipped v3.11 ontology layout.
--
-- ckp.boot() IS the documented arming step — pgCK's own fresh-install contract
-- (scripts/smoke-s34-fresh-install.sh, "(2b) the documented arming step: boot +
-- module import from /ontology") runs exactly this on a virgin cluster. It is
-- operator-level at first boot, NOT a ckp.dispatch verb, so ck_participant still
-- cannot write /ck and the Critical Isolation floor holds.
--
-- REPLACES the v3.9-era pair `CALL ckp.bootstrap_kernel()` + two
-- ckp.adopt_kernel_ttl(pg_read_file('/ontology/{task,goal}.ttl'), 'demo') calls.
-- Both TTLs are GONE from the artifact: CKP v3.11 restructured /ontology into a
-- versioned tree (v3.11/core.ttl + v3.11/modules/{wave,lexicon}.ttl), and pgCK
-- 0.4.40 RETIRED the ckp:Task / ckp:Goal board pair outright — they do not exist
-- in the v3.11 root. Reading those paths would now fail on a missing file, so
-- first boot would arm nothing and every typed-edge gate would go vacuous again
-- (the #18 regression v0.7.20 was cut to fix).
--
-- Under v3.11 a module reaches a surface ONLY through a sealed ckp:Adoption
-- naming its digest, so there is deliberately no demo-shape import here: the
-- shipped layout is loaded, and adoption is a sealed act, not a boot-time side
-- effect.
CALL ckp.boot();

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
-- One line, no embedded newlines: `postgres --single` terminates a command on
-- a bare newline (not just `;`), so a multi-line DO $$ ... $$ block here gets
-- split mid-body and fails with "unterminated dollar-quoted string" — measured
-- 2026-08-19 smoking this file through the real launcher path.
DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'ck_drainer') THEN CREATE ROLE ck_drainer LOGIN; END IF; END $$;
GRANT USAGE ON SCHEMA ckp TO ck_drainer;
GRANT SELECT, DELETE ON ckp.outbox TO ck_drainer;
