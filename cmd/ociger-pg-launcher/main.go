package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/sporaxis-com/oci-germination/internal/launcher"
)

const (
	pgMajor     = "17"
	postgresUID = 999
	postgresGID = 999
)

func main() {
	pgData := getenv("PGDATA", "/var/lib/postgresql/data")
	socketDir := "/var/run/postgresql"

	must(os.MkdirAll(filepath.Dir(pgData), 0o755))
	must(os.MkdirAll(pgData, 0o700))
	must(os.MkdirAll(socketDir, 0o775))
	must(os.Chmod(pgData, 0o700))
	must(os.Chmod(socketDir, 0o775))

	stat, err := os.Stat(pgData)
	must(err)
	sysStat, ok := stat.Sys().(*syscall.Stat_t)
	if !ok {
		log.Fatal("pgdata stat did not expose unix ownership")
	}

	runUID, runGID, err := launcher.SelectIdentity(
		postgresUID,
		postgresGID,
		int(sysStat.Uid),
		int(sysStat.Gid),
		os.Chown(pgData, postgresUID, postgresGID),
	)
	must(err)
	must(os.Chown(socketDir, runUID, runGID))
	must(launcher.EnsureIdentityFiles("/etc/passwd", "/etc/group", runUID, runGID))

	if _, err := os.Stat(filepath.Join(pgData, "PG_VERSION")); os.IsNotExist(err) {
		runAsPostgres(
			runUID,
			runGID,
			bin("initdb"),
			"-D", pgData,
			"--username=postgres",
			"--auth=trust",
			"--locale=C",
			"--encoding=UTF8",
		)
		appendFile(filepath.Join(pgData, "postgresql.conf"), "listen_addresses='*'\nunix_socket_directories='/var/run/postgresql'\n")
		// OCIGER_CK_PARTICIPANT_HBA: when set to a non-empty value, prepend a
		// per-role pg_hba.conf line for ck_participant requiring scram-sha-256
		// BEFORE the catch-all `host all all all trust`. pg_hba is first-match
		// wins, so the order matters: ck_participant must be matched by the
		// scram rule, not the trust catch-all. Without this env, the launcher
		// behaves exactly as v0.1.7 (trust everyone — dev-mode).
		if os.Getenv("OCIGER_CK_PARTICIPANT_HBA") != "" {
			appendFile(filepath.Join(pgData, "pg_hba.conf"), "host all ck_participant 0.0.0.0/0 scram-sha-256\n")
		}
		appendFile(filepath.Join(pgData, "pg_hba.conf"), "host all all all trust\n")
		// OCIGER_POSTGRES_CONF_EXTRA: any extra postgresql.conf lines a bundle
		// wants baked in at first initdb. ck-allinone uses this to set
		// pgck.nats_url BEFORE postgres starts (pgCK's bgworker only reads
		// the GUC once on first tick — SIGHUP after-the-fact doesn't wake it).
		if extra := os.Getenv("OCIGER_POSTGRES_CONF_EXTRA"); extra != "" {
			if !strings.HasSuffix(extra, "\n") {
				extra += "\n"
			}
			appendFile(filepath.Join(pgData, "postgresql.conf"), extra)
		}
		// OCIGER_INITDB_SQL_FILE: a SQL script piped through postgres single-user
		// mode immediately after initdb, before the normal server starts. Lets
		// a bundle issue CREATE EXTENSION + similar bootstrap statements without
		// shipping a postgres client (psql) in the final image. Idempotent by
		// virtue of the surrounding "PG_VERSION absent → first-boot" gate.
		if sqlFile := os.Getenv("OCIGER_INITDB_SQL_FILE"); sqlFile != "" {
			sql, err := os.ReadFile(sqlFile)
			if err != nil {
				log.Fatalf("OCIGER_INITDB_SQL_FILE %q: %v", sqlFile, err)
			}
			sqlStr := string(sql)
			// OCIGER_CK_PARTICIPANT_PASSWORD: when set, append an ALTER ROLE
			// statement to the bootstrap SQL so ck_participant becomes a login
			// role with a scram-sha-256 password. Required for the v3.9 Critical
			// Isolation Alpha; without it, ck_participant exists but cannot log
			// in. The password is single-quote-escaped before embedding. If
			// unset, we log a warning (the bundle README says deploys MUST set
			// it before external exposure).
			if pwd := os.Getenv("OCIGER_CK_PARTICIPANT_PASSWORD"); pwd != "" {
				quoted := strings.ReplaceAll(pwd, "'", "''")
				sqlStr += fmt.Sprintf("\nALTER ROLE ck_participant WITH LOGIN PASSWORD '%s';\n", quoted)
				log.Println("ociger-pg-launcher: ck_participant configured with deploy-supplied password")
			} else if os.Getenv("OCIGER_CK_PARTICIPANT_HBA") != "" {
				log.Println("ociger-pg-launcher: WARNING — OCIGER_CK_PARTICIPANT_HBA set but OCIGER_CK_PARTICIPANT_PASSWORD unset; ck_participant cannot log in until you set it")
			}
			runAsPostgresStdin(runUID, runGID, sqlStr,
				bin("postgres"), "--single", "-D", pgData, "postgres")
		}
	}

	args := launcher.PostgresArgs(pgData, os.Getenv("OCIGER_SHARED_PRELOAD_LIBRARIES"))
	dropPrivilegesAndExec(runUID, runGID, args[0], args[1:]...)
}

func bin(name string) string {
	return filepath.Join("/usr/lib/postgresql", pgMajor, "bin", name)
}

func runAsPostgres(uid int, gid int, name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid: uint32(uid),
			Gid: uint32(gid),
		},
	}
	if err := cmd.Run(); err != nil {
		log.Fatalf("%s failed: %v", name, err)
	}
}

func runAsPostgresStdin(uid int, gid int, stdin string, name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = strings.NewReader(stdin)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid: uint32(uid),
			Gid: uint32(gid),
		},
	}
	if err := cmd.Run(); err != nil {
		log.Fatalf("%s failed: %v", name, err)
	}
}

func dropPrivilegesAndExec(uid int, gid int, name string, args ...string) {
	must(syscall.Setgroups([]int{gid}))
	must(syscall.Setgid(gid))
	must(syscall.Setuid(uid))
	must(syscall.Exec(name, append([]string{name}, args...), os.Environ()))
}

func appendFile(path string, contents string) {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	if _, err := fmt.Fprint(file, contents); err != nil {
		log.Fatal(err)
	}
}

func getenv(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
