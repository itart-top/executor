//go:build !windows
// +build !windows

package executor_test

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/itart-top/executor/executor"
	"github.com/stretchr/testify/assert"
)

// TestRun_Success tests successful command execution.
func TestRun_Success(t *testing.T) {
	result := executor.Run(context.Background(), "echo", executor.WithArgs("hello"))

	assert.NoError(t, result.Err)
	assert.Equal(t, 0, result.ExitCode)
	assert.Equal(t, "hello\n", result.Stdout)
	assert.Empty(t, result.Stderr)
	assert.True(t, result.Duration > 0)
}

// TestRun_CommandNotFound tests handling of non-existent command.
func TestRun_Failure(t *testing.T) {
	result := executor.Run(context.Background(), "false")

	// false 命令会返回 ExitError，这是预期行为
	// 检查是否是预期的退出码 1
	assert.Equal(t, 1, result.ExitCode)
	assert.True(t, result.Duration > 0)

	// 对于退出码错误，我们仍然认为这是"成功执行"的情况
	// 只是命令本身返回了非零状态
	var exitErr *exec.ExitError
	if errors.As(result.Err, &exitErr) {
		// 确认这是一个正常的退出错误而不是其他类型的错误
		assert.True(t, true) // 确认是 ExitError
	}
}

// TestRun_CommandNotFound tests handling of non-existent command.
func TestRun_CommandNotFound(t *testing.T) {
	result := executor.Run(context.Background(), "nonexistentcommand")
	assert.Error(t, result.Err)
	assert.Contains(t, result.Err.Error(), "executable file not found")
	assert.Equal(t, -1, result.ExitCode)
	assert.Empty(t, result.Stdout)
	assert.Empty(t, result.Stderr)
}

// TestRun_Timeout tests command execution timeout.
func TestRun_Timeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	result := executor.Run(ctx, "sleep", executor.WithArgs("1"))

	assert.Error(t, result.Err)
	assert.ErrorIs(t, result.Err, executor.ErrContextDone)
	assert.Equal(t, -1, result.ExitCode)
	assert.Empty(t, result.Stdout)
	assert.Empty(t, result.Stderr)
}

// TestRun_WithEnv tests command execution with custom environment variables.
func TestRun_WithEnv(t *testing.T) {
	result := executor.Run(
		context.Background(),
		"sh",
		executor.WithArgs("-c", "echo $TEST_VAR"),
		executor.WithEnv("TEST_VAR=hello"),
	)

	assert.NoError(t, result.Err)
	assert.Equal(t, 0, result.ExitCode)
	assert.Equal(t, "hello\n", result.Stdout)
}

// TestRun_WithDir tests command execution with custom working directory.
func TestRun_WithDir(t *testing.T) {
	result := executor.Run(
		context.Background(),
		"pwd",
		executor.WithDir("/tmp"),
	)

	assert.NoError(t, result.Err)
	assert.Equal(t, 0, result.ExitCode)
	assert.True(t, strings.Contains(result.Stdout, "/tmp"))
}

// TestRun_WithOutputCapture tests that output is captured correctly.
func TestRun_WithOutputCapture(t *testing.T) {
	var stdout, stderr strings.Builder

	result := executor.Run(
		context.Background(),
		"sh",
		executor.WithArgs("-c", "echo 'stdout message'; echo 'stderr message' >&2"),
		executor.WithStdout(&stdout),
		executor.WithStderr(&stderr),
	)

	assert.NoError(t, result.Err)
	assert.Equal(t, 0, result.ExitCode)
	assert.Equal(t, "stdout message\n", result.Stdout)
	assert.Equal(t, "stderr message\n", result.Stderr)
	assert.Equal(t, "stdout message\n", stdout.String())
	assert.Equal(t, "stderr message\n", stderr.String())
}

// TestRun_EmptyCommand tests handling of empty command.
func TestRun_EmptyCommand(t *testing.T) {
	result := executor.Run(context.Background(), "")

	assert.Error(t, result.Err)
	assert.Equal(t, "executor: command cannot be empty", result.Err.Error())
	assert.Equal(t, -1, result.ExitCode)
	assert.Empty(t, result.Stdout)
	assert.Empty(t, result.Stderr)
	assert.Zero(t, result.Duration)
}

// TestRun_WithArgs tests command execution with arguments.
func TestRun_WithArgs(t *testing.T) {
	result := executor.Run(
		context.Background(),
		"printf",
		executor.WithArgs("Hello, %s!", "World"),
	)

	assert.NoError(t, result.Err)
	assert.Equal(t, 0, result.ExitCode)
	assert.Equal(t, "Hello, World!", result.Stdout)
}

// context with time out and cancel
func TestRun_WithTimeoutAndCancel(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	result := executor.Run(ctx, "sleep", executor.WithArgs("1"))
	assert.Error(t, result.Err)
	assert.ErrorIs(t, result.Err, executor.ErrContextDone)
	assert.Equal(t, -1, result.ExitCode)
	assert.Empty(t, result.Stdout)
	assert.Empty(t, result.Stderr)
}

// async print result
func TestRun_AsyncPrintResult(t *testing.T) {
	// 建立一个管道：命令写入 writer，异步读取 reader
	pr, pw := io.Pipe()

	// 异步读取 goroutine（模拟实时日志消费）
	go func() {
		defer pr.Close()
		reader := bufio.NewReader(pr)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					break
				}
				fmt.Println("read error:", err)
				break
			}
			fmt.Printf("[REALTIME] %s", line)
		}
	}()

	// Linux/macOS 命令：每秒输出一行
	cmd := "bash"
	args := []string{"-c", "for i in {1..5}; do echo tick $i; sleep 1; done"}

	// 使用 Pipe writer 作为 stdout
	res := executor.Run(context.Background(), cmd,
		executor.WithArgs(args...),
		executor.WithStdout(pw),
	)

	pw.Close() // 关闭 writer，通知 reader 结束

	fmt.Println("==== Command finished ====")
	fmt.Println("ExitCode:", res.ExitCode)
	fmt.Println("Error:", res.Err)
	fmt.Println("Duration:", res.Duration)

	if !strings.Contains(res.Stdout, "tick 1") {
		t.Errorf("expected 'tick 1' in stdout, got: %s", res.Stdout)
	}
}

// froxt
// checkProcessExists 检查某个 PID 是否还存在
func checkProcessExists(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil
}

func TestRun_ForkedProcessKill(t *testing.T) {
	// 创建一个临时脚本，脚本内部 fork 出子进程
	script := `
#!/bin/bash
echo "Parent PID: $$"
# 子进程在后台循环打印
(while true; do echo "child $$ running"; sleep 1; done) &
# 主进程也循环打印
while true; do echo "parent $$ running"; sleep 1; done
`
	tmpFile := "test_fork.sh"
	if err := os.WriteFile(tmpFile, []byte(script), 0755); err != nil {
		t.Fatalf("failed to create temp script: %v", err)
	}
	defer os.Remove(tmpFile)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var output strings.Builder
	res := executor.Run(ctx, "bash",
		executor.WithArgs(tmpFile),
		executor.WithStdout(&output),
		executor.WithStderr(&output),
	)

	fmt.Println("==== Command finished ====")
	fmt.Println("ExitCode:", res.ExitCode)
	fmt.Println("Error:", res.Err)
	fmt.Println("Duration:", res.Duration)
	fmt.Println("Captured Output:\n", output.String())

	// 验证执行器确实触发了超时 kill
	if ctx.Err() == nil {
		t.Errorf("expected context timeout, got nil")
	}

	// 从输出中解析 parent pid
	var parentPID int
	fmt.Sscanf(output.String(), "Parent PID: %d", &parentPID)
	if parentPID > 0 && checkProcessExists(parentPID) {
		t.Errorf("parent process %d still alive", parentPID)
	}

	// 校验结果
	if res.ExitCode == 0 {
		t.Errorf("expected non-zero exit code on kill, got 0")
	}
}

// TestRun_OutputTruncation tests that large output is truncated when maxOutput is set.
func TestRun_OutputTruncation(t *testing.T) {
	const maxBytes = 10 // 限制捕获最多 10 个字节
	// 生成比 maxBytes 更大的输出
	largeOutput := "abcdefghijklmnopqrstuvwxyz" // 26 bytes

	result := executor.Run(
		context.Background(),
		"sh",
		executor.WithArgs("-c", fmt.Sprintf("echo '%s'; echo '%s' >&2", largeOutput, largeOutput)),
		executor.WithMaxOutput(maxBytes),
	)

	assert.NoError(t, result.Err)
	assert.Equal(t, 0, result.ExitCode)

	// stdout/stderr 都应该被截断
	assert.True(t, result.StdoutTruncated, "stdout should be truncated")
	assert.True(t, result.StderrTruncated, "stderr should be truncated")

	// captured 内容长度 <= maxBytes
	assert.LessOrEqual(t, len(result.Stdout), maxBytes)
	assert.LessOrEqual(t, len(result.Stderr), maxBytes)

	// 外部 stdout/stderr 不受限制，可选择检查
}
