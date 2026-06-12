-- Auto-create pgRDF on first boot. The stock postgres entrypoint runs every
-- /docker-entrypoint-initdb.d/*.sql against a transient server during initdb
-- (first boot only, on a fresh volume). Because the image baked
-- shared_preload_libraries = 'pgrdf' into the conf template, that transient
-- server already has pgrdf loaded, so this CREATE EXTENSION succeeds and the
-- real server inherits the installed extension — nothing to run after start.
CREATE EXTENSION IF NOT EXISTS pgrdf;
