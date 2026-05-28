package main

import (
	"errors"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/sporaxis-com/oci-germination/internal/supervisor"
)

type childResult struct {
	name string
	cmd  *exec.Cmd
	err  error
}

func main() {
	exitCode := run(supervisor.DefaultPrograms(), realStarter, log.Default())
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

// starter abstracts how to spawn a Program's process. Production uses
// realStarter (exec.Cmd via /usr/local/bin/<binary>). Tests inject a
// fake starter that runs in-process commands.
type starter func(p supervisor.Program) (*exec.Cmd, error)

func realStarter(p supervisor.Program) (*exec.Cmd, error) {
	cmd := exec.Command(p.Path, p.Args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

// run starts every program via the starter, waits for the first to exit
// or a signal, terminates the rest, reaps everyone, and returns the exit
// code. Exposed for testing. Returns 0 on clean shutdown.
func run(programs []supervisor.Program, start starter, logger *log.Logger) int {
	cmds := make([]*exec.Cmd, 0, len(programs))
	for _, program := range programs {
		cmd, err := start(program)
		if err != nil {
			if logger != nil {
				logger.Printf("start %s: %v", program.Name, err)
			}
			return 1
		}
		cmds = append(cmds, cmd)
	}

	results := make(chan childResult, len(programs))
	for i, cmd := range cmds {
		program := programs[i]
		go func(name string, cmd *exec.Cmd) {
			results <- childResult{name: name, cmd: cmd, err: cmd.Wait()}
		}(program.Name, cmd)
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signals)

	terminated := make(map[int]struct{}, len(cmds))
	intentional := make(map[int]struct{}, len(cmds))
	exitCode := 0

	select {
	case sig := <-signals:
		if logger != nil {
			logger.Printf("received %s, shutting down", sig)
		}
		terminateSiblings(cmds, nil, terminated, intentional)
	case result := <-results:
		if code := unexpectedExitCode(result, intentional); code != 0 {
			if logger != nil {
				logger.Printf("%s exited: %v", result.name, result.err)
			}
			exitCode = code
		} else if logger != nil {
			logger.Printf("%s exited", result.name)
		}
		terminated[result.cmd.Process.Pid] = struct{}{}
		terminateSiblings(cmds, result.cmd, terminated, intentional)
	}

	remaining := len(cmds) - len(terminated)
	for remaining > 0 {
		result := <-results
		remaining--
		terminated[result.cmd.Process.Pid] = struct{}{}
		if exitCode == 0 {
			if code := unexpectedExitCode(result, intentional); code != 0 {
				if logger != nil {
					logger.Printf("%s exited while shutting down: %v", result.name, result.err)
				}
				exitCode = code
			}
		}
	}

	return exitCode
}

func terminateSiblings(cmds []*exec.Cmd, exclude *exec.Cmd, terminated map[int]struct{}, intentional map[int]struct{}) {
	for _, cmd := range cmds {
		if cmd == exclude || cmd.Process == nil {
			continue
		}
		pid := cmd.Process.Pid
		if _, ok := terminated[pid]; ok {
			continue
		}
		if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil {
			if !errors.Is(err, syscall.ESRCH) {
				log.Printf("terminate pid %d: %v", pid, err)
			}
			continue
		}
		intentional[pid] = struct{}{}
	}
}

func unexpectedExitCode(result childResult, intentional map[int]struct{}) int {
	if result.err == nil {
		return 0
	}
	if result.cmd != nil && result.cmd.Process != nil {
		if _, ok := intentional[result.cmd.Process.Pid]; ok {
			return 0
		}
	}

	code := exitStatus(result.err)
	if code == 0 {
		return 1
	}
	return code
}

func exitStatus(err error) int {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return 1
	}

	status, ok := exitErr.Sys().(syscall.WaitStatus)
	if !ok {
		return 1
	}
	if status.Exited() {
		return status.ExitStatus()
	}
	if status.Signaled() {
		return 128 + int(status.Signal())
	}
	return 1
}
