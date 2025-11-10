//go:build windows
// +build windows

package executor

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"
)

// Credential is unsupported on Windows (placeholder for API consistency).
type Credential struct {
	UID uint32
	GID uint32
}

func runCommand(ctx context.Context, cfg *Config) Result {
	execCmd := exec.CommandContext(ctx, cfg.cmd, cfg.args...)
	execCmd.Dir = cfg.dir
	execCmd.Env = append(os.Environ(), cfg.env...)

	// Windows doesn't support syscall.Credential or pgid.
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
			_ = execCmd.Process.Kill()
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
