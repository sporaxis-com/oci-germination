package main

import (
	"os/exec"
	"syscall"
	"testing"
	"time"
)

func TestUnexpectedExitCode(t *testing.T) {
	t.Run("returns zero for clean exit", func(t *testing.T) {
		result := runCommand(t, "exit 0")

		if got := unexpectedExitCode(result, nil); got != 0 {
			t.Fatalf("unexpectedExitCode() = %d, want 0", got)
		}
	})

	t.Run("returns zero for intentionally terminated sibling", func(t *testing.T) {
		cmd := exec.Command("bash", "-lc", "sleep 30")
		if err := cmd.Start(); err != nil {
			t.Fatalf("Start() returned error: %v", err)
		}
		t.Cleanup(func() {
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
		})

		time.Sleep(100 * time.Millisecond)
		if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
			t.Fatalf("Signal() returned error: %v", err)
		}

		result := childResult{
			name: "nats",
			cmd:  cmd,
			err:  cmd.Wait(),
		}
		intentional := map[int]struct{}{cmd.Process.Pid: {}}

		if got := unexpectedExitCode(result, intentional); got != 0 {
			t.Fatalf("unexpectedExitCode() = %d, want 0", got)
		}
	})

	t.Run("returns non-zero for unexpected child failure", func(t *testing.T) {
		result := runCommand(t, "exit 7")

		if got := unexpectedExitCode(result, nil); got != 7 {
			t.Fatalf("unexpectedExitCode() = %d, want 7", got)
		}
	})
}

func runCommand(t *testing.T, script string) childResult {
	t.Helper()

	cmd := exec.Command("bash", "-lc", script)
	if err := cmd.Start(); err != nil {
		t.Fatalf("Start() returned error: %v", err)
	}

	return childResult{
		name: "test",
		cmd:  cmd,
		err:  cmd.Wait(),
	}
}
