//go:build !windows
// +build !windows

package executor

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// Credential represents UID/GID to run the command with (Unix only).
type Credential struct {
	UID uint32
	GID uint32
}

func runCommand(ctx context.Context, cfg *Config) Result {
	execCmd := exec.CommandContext(ctx, cfg.cmd, cfg.args...)
	execCmd.Dir = cfg.dir
	execCmd.Env = append(os.Environ(), cfg.env...)

	sysAttr := &syscall.SysProcAttr{Setpgid: true}
	if cfg.credential != nil {
		sysAttr.Credential = &syscall.Credential{
			Uid: cfg.credential.UID,
			Gid: cfg.credential.GID,
		}
	}
	execCmd.SysProcAttr = sysAttr

	// capture buffers
	stdoutBuf := newLimitedBuffer(cfg.maxOutput)
	stderrBuf := newLimitedBuffer(cfg.maxOutput)
	execCmd.Stdout = io.MultiWriter(stdoutBuf, cfg.stdout)
	execCmd.Stderr = io.MultiWriter(stderrBuf, cfg.stderr)

	start := time.Now()
	if err := execCmd.Start(); err != nil {
		return Result{Err: fmt.Errorf("%w: %v", ErrStart, err), ExitCode: -1}
	}

	done := make(chan error, 1)
	go func() { done <- execCmd.Wait() }()

	var waitErr error
	select {
	case <-ctx.Done():
		if execCmd.Process != nil {
			_ = syscall.Kill(-execCmd.Process.Pid, syscall.SIGKILL)
		}
		waitErr = <-done
		waitErr = fmt.Errorf("%w: %v", ErrContextDone, waitErr)
	case err := <-done:
		waitErr = err
	}

	duration := time.Since(start)
	exitCode := -1
	if execCmd.ProcessState != nil {
		exitCode = execCmd.ProcessState.ExitCode()
	} else if err := waitErr; err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				if ws.Exited() {
					exitCode = ws.ExitStatus()
				} else if ws.Signaled() {
					exitCode = 128 + int(ws.Signal())
				}
			}
		}
	}

	return Result{
		Stdout:          stdoutBuf.String(),
		Stderr:          stderrBuf.String(),
		ExitCode:        exitCode,
		Err:             waitErr,
		Duration:        duration,
		StdoutTruncated: stdoutBuf.truncated,
		StderrTruncated: stderrBuf.truncated,
	}
}
