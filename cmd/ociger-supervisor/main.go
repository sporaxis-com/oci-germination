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
	programs := supervisor.DefaultPrograms()
	cmds := make([]*exec.Cmd, 0, len(programs))
	for _, program := range programs {
		cmd := exec.Command(program.Path, program.Args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = nil
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		if err := cmd.Start(); err != nil {
			log.Fatalf("start %s: %v", program.Name, err)
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
	exitCode := 0
	shutdownBySignal := false

	select {
	case sig := <-signals:
		log.Printf("received %s, shutting down", sig)
		shutdownBySignal = true
		terminateSiblings(cmds, nil, terminated)
	case result := <-results:
		if result.err != nil {
			log.Printf("%s exited: %v", result.name, result.err)
			exitCode = exitStatus(result.err)
			if exitCode == 0 {
				exitCode = 1
			}
		} else {
			log.Printf("%s exited", result.name)
		}
		terminated[result.cmd.Process.Pid] = struct{}{}
		terminateSiblings(cmds, result.cmd, terminated)
	}

	remaining := len(cmds) - len(terminated)
	for remaining > 0 {
		result := <-results
		remaining--
		terminated[result.cmd.Process.Pid] = struct{}{}
		if !shutdownBySignal && result.err != nil && exitCode == 0 {
			log.Printf("%s exited while shutting down: %v", result.name, result.err)
			exitCode = exitStatus(result.err)
			if exitCode == 0 {
				exitCode = 1
			}
		}
	}

	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

func terminateSiblings(cmds []*exec.Cmd, exclude *exec.Cmd, terminated map[int]struct{}) {
	for _, cmd := range cmds {
		if cmd == exclude || cmd.Process == nil {
			continue
		}
		pid := cmd.Process.Pid
		if _, ok := terminated[pid]; ok {
			continue
		}
		if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
			log.Printf("terminate pid %d: %v", pid, err)
		}
	}
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
