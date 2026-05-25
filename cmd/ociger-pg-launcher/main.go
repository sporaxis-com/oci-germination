package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
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
	must(os.Chown(pgData, postgresUID, postgresGID))
	must(os.Chown(socketDir, postgresUID, postgresGID))

	if _, err := os.Stat(filepath.Join(pgData, "PG_VERSION")); os.IsNotExist(err) {
		runAsPostgres(
			bin("initdb"),
			"-D", pgData,
			"--username=postgres",
			"--auth=trust",
			"--locale=C",
			"--encoding=UTF8",
		)
		appendFile(filepath.Join(pgData, "postgresql.conf"), "listen_addresses='*'\nunix_socket_directories='/var/run/postgresql'\n")
		appendFile(filepath.Join(pgData, "pg_hba.conf"), "host all all all trust\n")
	}

	dropPrivilegesAndExec(bin("postgres"), "-D", pgData)
}

func bin(name string) string {
	return filepath.Join("/usr/lib/postgresql", pgMajor, "bin", name)
}

func runAsPostgres(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid: postgresUID,
			Gid: postgresGID,
		},
	}
	if err := cmd.Run(); err != nil {
		log.Fatalf("%s failed: %v", name, err)
	}
}

func dropPrivilegesAndExec(name string, args ...string) {
	must(syscall.Setgroups([]int{postgresGID}))
	must(syscall.Setgid(postgresGID))
	must(syscall.Setuid(postgresUID))
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
