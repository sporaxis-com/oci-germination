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

-- pgCK v0.4.x design: the durable per-kernel tables (ckp.instances, ledger,
-- proof, outbox) are NOT created at install time — they're created inside
-- the stored procedure ckp.bootstrap_kernel(). The CREATE EXTENSION above
-- installs the procedure but does not call it; without this CALL, every
-- ckp.dispatch invocation that touches a sealed instance fails with
-- `relation "ckp.instances" does not exist`. The procedure is idempotent
-- (CREATE TABLE IF NOT EXISTS throughout) so calling it on first boot is
-- safe and required.
--
-- This was the underlying cause of v0.7.5..v0.7.13 silently shipping a
-- non-functional dispatch path that CK.Lib.Js verify-v150 caught as
-- "verb not governed yet (CI-B)" — the 4-arg dispatch stub returns that
-- envelope for every verb regardless, masking the real issue. v0.7.14
-- switches the relay to the GOVERNED 2-arg dispatch AND CALLs
-- bootstrap_kernel here, so the seal handlers actually have tables to
-- write to and verify-v150 should now see ok:true + proof_digest.
CALL ckp.bootstrap_kernel();

-- pgCK 0.4.1 grant-order gap: the CI-A role floor in the CREATE EXTENSION step
-- grants on tables that exist AT INSTALL TIME, but the per-kernel durable
-- tables (instances, ledger, proof, outbox) are created LAZILY by the
-- bootstrap_kernel() procedure above — AFTER the install-time grant pass.
-- The result is that ck_substrate (the SECURITY DEFINER subject of
-- ckp.dispatch) cannot read or write those tables, and every governed
-- dispatch errors with `relation "ckp.instances" does not exist`.
--
-- The fix re-applies the role floor to the just-created tables. NOTIFY filed
-- upstream proposing pgCK either (a) create the tables at install time so the
-- existing grant pass picks them up, or (b) re-apply the floor inside
-- bootstrap_kernel(). Until that ships, the lines below close the gap.
ALTER TABLE ckp.instances OWNER TO ck_substrate;
ALTER TABLE ckp.ledger    OWNER TO ck_substrate;
ALTER TABLE ckp.proof     OWNER TO ck_substrate;
ALTER TABLE ckp.outbox    OWNER TO ck_substrate;
GRANT USAGE ON SCHEMA pgrdf TO ck_substrate, ck_participant;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA pgrdf TO ck_substrate, ck_participant;
