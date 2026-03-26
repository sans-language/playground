package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type limitedWriter struct {
	buf bytes.Buffer
	max int
}

func (w *limitedWriter) Write(p []byte) (int, error) {
	remaining := w.max - w.buf.Len()
	if remaining <= 0 {
		return len(p), nil // silently discard
	}
	if len(p) > remaining {
		p = p[:remaining]
	}
	return w.buf.Write(p)
}

func (w *limitedWriter) String() string {
	return w.buf.String()
}

type RunResult struct {
	Stdout         string `json:"stdout"`
	Stderr         string `json:"stderr"`
	ExitCode       int    `json:"exit_code"`
	CompileSuccess bool   `json:"compile_success"`
}

const (
	runTimeout = 10 * time.Second
	imageName  = "sans-playground"
)

func runCode(code string) RunResult {
	// Acquire semaphore
	runSem <- struct{}{}
	defer func() { <-runSem }()

	tmpDir, err := os.MkdirTemp("", "sans-play-*")
	if err != nil {
		return RunResult{Stderr: "internal error: " + err.Error(), ExitCode: 1}
	}
	defer os.RemoveAll(tmpDir)

	codePath := filepath.Join(tmpDir, "code.sans")
	if err := os.WriteFile(codePath, []byte(code), 0644); err != nil {
		return RunResult{Stderr: "internal error: " + err.Error(), ExitCode: 1}
	}

	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
	defer cancel()

	containerName := fmt.Sprintf("sans-run-%d", time.Now().UnixNano())

	stdout := &limitedWriter{max: 65536}
	stderr := &limitedWriter{max: 65536}
	cmd := exec.CommandContext(ctx, "docker", "run",
		"--rm",
		"--name", containerName,
		"--network=none",
		"--memory=256m",
		"--cpus=1",
		"--pids-limit=64",
		"--security-opt=no-new-privileges",
		"--ulimit", "fsize=10485760:10485760",
		"--ulimit", "nofile=64:64",
		"--ulimit", "core=0:0",
		"-v", codePath+":/tmp/code.sans:ro",
		imageName,
		"sh", "-c", "cp /tmp/code.sans /home/runner/code.sans && sans build /home/runner/code.sans -o /home/runner/code 2>/home/runner/build.err > /dev/null; if [ -x /home/runner/code ]; then /home/runner/code; else grep 'error:' /home/runner/build.err >&2; exit 1; fi",
	)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err = cmd.Run()

	result := RunResult{
		Stdout: truncate(stdout.String(), 64*1024),
		Stderr: truncate(stderr.String(), 64*1024),
	}

	if ctx.Err() == context.DeadlineExceeded {
		exec.Command("docker", "rm", "-f", containerName).Run()
		result.Stderr = "execution timed out (10s limit)"
		result.ExitCode = 124
		result.CompileSuccess = false
		return result
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = 1
		}
	}

	result.CompileSuccess = result.ExitCode == 0 || !strings.Contains(stderr.String(), "error:")
	if result.ExitCode == 0 {
		result.CompileSuccess = true
	}

	return result
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + fmt.Sprintf("\n... (truncated, %d bytes total)", len(s))
}
