## 🚀 项目简介

`executor` 是一个 **高性能、跨平台的 Go 命令执行器**，为开发者提供可靠、可控的命令/脚本执行环境。它解决了以下痛点：

- 需要安全执行外部命令和脚本
- 实时捕获 stdout/stderr，进行日志处理
- 防止超时命令阻塞程序
- 自动清理子进程及子孙进程
- 大量输出可能导致内存占用问题

无论是 **分布式任务执行、后台服务、自动化脚本**，还是 **CI/CD 工具链**，`executor` 都能让命令执行变得安全、高效、可控。

------

## 🌟 核心特性

- **跨平台支持**：Linux / macOS / Windows
- **实时输出捕获**：`io.Writer` 实时流式消费日志
- **超时 & 取消**：支持 `context.Context` 控制执行时长
- **子进程管理**：Unix 平台支持干净终止子进程及子孙进程
- **输出截断保护**：限制缓冲区大小，防止 OOM
- **易用 Option API**：可组合配置参数，无需修改函数签名

------

## ⚡ 快速开始

### 安装

```shell
go get github.com/itart-top/executor
```

### 简单示例

```go
package main

import (
	"context"
	"fmt"

	"github.com/itart-top/executor"
)

func main() {
	result := executor.Run(
		context.Background(),
		"echo",
		executor.WithArgs("Hello, Executor!"),
	)

	fmt.Println("ExitCode:", result.ExitCode)
	fmt.Println("Stdout:", result.Stdout)
	fmt.Println("Stderr:", result.Stderr)
}
```

------

## 🛠 进阶用法

### 1. 捕获 stdout/stderr

```go
var stdout, stderr strings.Builder

result := executor.Run(
	context.Background(),
	"sh",
	executor.WithArgs("-c", "echo 'stdout message'; echo 'stderr message' >&2"),
	executor.WithStdout(&stdout),
	executor.WithStderr(&stderr),
)

fmt.Println("Stdout:", result.Stdout)
fmt.Println("Stderr:", result.Stderr)
```

### 2. 设置工作目录和环境变量

```go
result := executor.Run(
	context.Background(),
	"sh",
	executor.WithArgs("-c", "echo $MY_VAR"),
	executor.WithEnv("MY_VAR=hello"),
	executor.WithDir("/tmp"),
)
fmt.Println(result.Stdout) // 输出 hello
```

### 3. 超时控制

```go
ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
defer cancel()

result := executor.Run(ctx, "sleep", executor.WithArgs("10"))
if result.Err != nil {
	fmt.Println("Timeout or cancel:", result.Err)
}
```

### 4. 限制输出大小

```go
result := executor.Run(
	context.Background(),
	"sh",
	executor.WithArgs("-c", "yes 'spam' | head -n 1000"),
	executor.WithMaxOutput(100), // 限制缓冲区最多 100 字节
)
fmt.Println("Truncated:", result.StdoutTruncated)
```

------

## 📚 API 说明

```go
func Run(ctx context.Context, cmd string, opts ...Option) Result
```

- `ctx`：上下文，可控制超时或取消
- `cmd`：命令名或路径
- `opts`：可选参数

### 常用 Option

- `WithArgs(args ...string)`
- `WithEnv(env ...string)`
- `WithDir(dir string)`
- `WithStdout(w io.Writer)`
- `WithStderr(w io.Writer)`
- `WithCredential(uid, gid uint32)` (Unix only)
- `WithMaxOutput(n int64)`

### Result 结构体

```go
type Result struct {
    Stdout           string
    Stderr           string
    ExitCode         int
    Err              error
    Duration         time.Duration
    StdoutTruncated  bool
    StderrTruncated  bool
}
```

------

## ✅ 测试

```
go test ./... -v
```

支持单元测试、截断测试、超时测试等。

------

## 🤝 贡献指南

1. Fork 本仓库
2. 新建分支 `feature/xxx`
3. 提交代码并发 Pull Request
4. 保持测试覆盖率，确保 `go test` 全部通过
5. 遵守 Go 官方代码规范

------

## 📄 License

MIT License © 2025 [itart]