// Package executor provides a robust command execution interface with support for
// real-time output, timeouts, and process management.
package executor

import (
	"bytes"
	"context"
	"errors"
	"io"
	"time"
)

// Config defines execution parameters.
type Config struct {
	cmd        string
	args       []string
	env        []string
	dir        string
	stdout     io.Writer
	stderr     io.Writer
	credential *Credential // platform-specific
	maxOutput  int64
}

// Option configures a command execution.
type Option func(*Config)

// --- Option helpers ---

func WithArgs(args ...string) Option {
	return func(c *Config) { c.args = args }
}

func WithEnv(env ...string) Option {
	return func(c *Config) { c.env = append(c.env, env...) }
}

func WithDir(dir string) Option {
	return func(c *Config) { c.dir = dir }
}

func WithStdout(w io.Writer) Option {
	return func(c *Config) { c.stdout = w }
}

func WithStderr(w io.Writer) Option {
	return func(c *Config) { c.stderr = w }
}

func WithCredential(uid, gid uint32) Option {
	return func(c *Config) { c.credential = &Credential{UID: uid, GID: gid} }
}

func WithMaxOutput(n int64) Option {
	return func(c *Config) { c.maxOutput = n }
}

// --- Errors ---

var (
	ErrContextDone = errors.New("executor: context timeout or cancelled")
	ErrStart       = errors.New("executor: failed to start")
)

// Result represents command execution result.
type Result struct {
	Stdout          string
	Stderr          string
	ExitCode        int
	Err             error
	Duration        time.Duration
	StdoutTruncated bool
	StderrTruncated bool
}

// --- limited buffer helper ---

type limitedBuffer struct {
	buf       bytes.Buffer
	limit     int64
	written   int64
	truncated bool
}

func newLimitedBuffer(limit int64) *limitedBuffer {
	return &limitedBuffer{limit: limit}
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.limit <= 0 {
		n, _ := b.buf.Write(p)
		b.written += int64(n)
		return len(p), nil
	}
	remaining := b.limit - b.written
	if remaining <= 0 {
		b.truncated = true
		return len(p), nil
	}
	if int64(len(p)) > remaining {
		n, _ := b.buf.Write(p[:remaining])
		b.written += int64(n)
		b.truncated = true
		return len(p), nil
	}
	n, _ := b.buf.Write(p)
	b.written += int64(n)
	return len(p), nil
}

func (b *limitedBuffer) String() string { return b.buf.String() }

// --- platform-agnostic runner ---

func Run(ctx context.Context, cmd string, opts ...Option) Result {
	if cmd == "" {
		return Result{Err: errors.New("executor: command cannot be empty"), ExitCode: -1}
	}

	cfg := &Config{
		cmd:       cmd,
		stdout:    io.Discard,
		stderr:    io.Discard,
		maxOutput: 1024 * 10,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	// platform-specific exec and kill logic handled in executor_unix.go / executor_windows.go
	return runCommand(ctx, cfg)
}
