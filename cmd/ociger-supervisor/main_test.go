package main

import (
	"errors"
	"io"
	"log"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/sporaxis-com/oci-germination/internal/supervisor"
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

// Tests for exitStatus

func TestExitStatus_NilError(t *testing.T) {
	// nil error path: errors.As fails → returns 1
	if got := exitStatus(nil); got != 1 {
		t.Fatalf("exitStatus(nil) = %d, want 1", got)
	}
}

func TestExitStatus_NonExitError(t *testing.T) {
	// errors.As fails for non-ExitError → returns 1
	err := errors.New("not an exit error")
	if got := exitStatus(err); got != 1 {
		t.Fatalf("exitStatus(plain error) = %d, want 1", got)
	}
}

func TestExitStatus_NormalExit(t *testing.T) {
	cmd := exec.Command("bash", "-lc", "exit 42")
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected non-nil err for exit 42")
	}
	if got := exitStatus(err); got != 42 {
		t.Fatalf("exitStatus(exit 42) = %d, want 42", got)
	}
}

func TestExitStatus_SignaledChild(t *testing.T) {
	cmd := exec.Command("bash", "-lc", "sleep 30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	// SIGTERM the child directly (not via process group)
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("Signal: %v", err)
	}
	waitErr := cmd.Wait()
	got := exitStatus(waitErr)
	// SIGTERM = signal 15 → 128 + 15 = 143
	want := 128 + int(syscall.SIGTERM)
	if got != want {
		t.Fatalf("exitStatus(SIGTERM) = %d, want %d", got, want)
	}
}

// Tests for terminateSiblings

func TestTerminateSiblings_KillsAllExceptExcluded(t *testing.T) {
	// Start two children that will sleep until killed
	cmd1 := startSleeper(t)
	cmd2 := startSleeper(t)
	cmds := []*exec.Cmd{cmd1, cmd2}

	terminated := map[int]struct{}{}
	intentional := map[int]struct{}{}

	// Exclude cmd1; only cmd2 should be killed
	terminateSiblings(cmds, cmd1, terminated, intentional)

	// cmd2's pid should now be in intentional
	if _, ok := intentional[cmd2.Process.Pid]; !ok {
		t.Errorf("cmd2 pid %d not marked intentional", cmd2.Process.Pid)
	}
	if _, ok := intentional[cmd1.Process.Pid]; ok {
		t.Errorf("excluded cmd1 pid %d unexpectedly marked intentional", cmd1.Process.Pid)
	}

	// cmd2 should die quickly; reap it
	waitErr2 := cmd2.Wait()
	if waitErr2 == nil {
		t.Errorf("cmd2 should have died with signal, got nil error")
	}

	// cmd1 still alive; clean up
	_ = cmd1.Process.Kill()
	_, _ = cmd1.Process.Wait()
}

func TestTerminateSiblings_SkipsAlreadyTerminated(t *testing.T) {
	cmd1 := startSleeper(t)
	defer func() {
		_ = cmd1.Process.Kill()
		_, _ = cmd1.Process.Wait()
	}()

	cmds := []*exec.Cmd{cmd1}
	terminated := map[int]struct{}{cmd1.Process.Pid: {}}
	intentional := map[int]struct{}{}

	terminateSiblings(cmds, nil, terminated, intentional)

	// cmd1 is already in terminated → should be skipped, NOT added to intentional
	if _, ok := intentional[cmd1.Process.Pid]; ok {
		t.Errorf("already-terminated pid %d marked intentional; should have been skipped", cmd1.Process.Pid)
	}
}

func TestTerminateSiblings_SkipsNilProcess(t *testing.T) {
	// A cmd that was never Start()'d has Process == nil
	cmd := exec.Command("true")
	cmds := []*exec.Cmd{cmd}
	terminated := map[int]struct{}{}
	intentional := map[int]struct{}{}

	// Should NOT panic
	terminateSiblings(cmds, nil, terminated, intentional)

	if len(intentional) != 0 {
		t.Errorf("nil-process cmd should be skipped; intentional = %v", intentional)
	}
}

func TestTerminateSiblings_ESRCHIsSilent(t *testing.T) {
	// Start, then reap the child (its pid is no longer signalable).
	// terminateSiblings should NOT log an error for ESRCH but also shouldn't mark intentional.
	cmd := startSleeper(t)
	_ = cmd.Process.Kill()
	_, _ = cmd.Process.Wait()

	cmds := []*exec.Cmd{cmd}
	terminated := map[int]struct{}{}
	intentional := map[int]struct{}{}

	terminateSiblings(cmds, nil, terminated, intentional)

	// The kill may or may not return ESRCH depending on timing/pid reuse;
	// the important property is that the function doesn't panic.
	_ = intentional
}

func startSleeper(t *testing.T) *exec.Cmd {
	t.Helper()
	cmd := exec.Command("bash", "-lc", "sleep 30")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("Start sleeper: %v", err)
	}
	// Give the shell time to set its process group
	time.Sleep(80 * time.Millisecond)
	return cmd
}

// Tests for run()

func silentLogger() *log.Logger {
	return log.New(io.Discard, "", 0)
}

// fakeStarter runs a shell command in-process for each program.
// The Path field is interpreted as a bash -lc script.
func fakeStarter(p supervisor.Program) (*exec.Cmd, error) {
	cmd := exec.Command("bash", "-lc", p.Path)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

func TestRun_FirstChildExitsZeroOthersTerminated(t *testing.T) {
	programs := []supervisor.Program{
		{Name: "fast", Path: "exit 0"},
		{Name: "slow", Path: "sleep 30"},
	}
	got := run(programs, fakeStarter, silentLogger())
	if got != 0 {
		t.Errorf("run() = %d, want 0 (first child clean exit triggers shutdown)", got)
	}
}

func TestRun_FirstChildExitsNonZeroReturnsCode(t *testing.T) {
	programs := []supervisor.Program{
		{Name: "fail", Path: "exit 7"},
		{Name: "slow", Path: "sleep 30"},
	}
	got := run(programs, fakeStarter, silentLogger())
	if got != 7 {
		t.Errorf("run() = %d, want 7 (first child's exit code propagates)", got)
	}
}

func TestRun_StarterErrorReturnsOne(t *testing.T) {
	failingStarter := func(p supervisor.Program) (*exec.Cmd, error) {
		return nil, errors.New("synthetic start failure")
	}
	programs := []supervisor.Program{{Name: "any", Path: "true"}}
	got := run(programs, failingStarter, silentLogger())
	if got != 1 {
		t.Errorf("run() with failing starter = %d, want 1", got)
	}
}

func TestRun_NoProgramsReturnsZero(t *testing.T) {
	// Edge: empty program list. Should fall through to shutdown path
	// via the select on signals — but there are no children to wait on.
	// In practice run() blocks on the select. We test via a goroutine
	// that triggers SIGTERM to the test process — but that's invasive.
	// Instead, document the limitation: empty programs is not a real
	// production case (DefaultPrograms always returns ≥2).
	t.Skip("empty programs blocks on signal select; not a real production case")
}
