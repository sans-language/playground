package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

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

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "docker", "run",
		"--rm",
		"--network=none",
		"--memory=256m",
		"--cpus=1",
		"--pids-limit=64",
		"--read-only",
		"--tmpfs=/home/runner:size=64m",
		"--tmpfs=/tmp:size=64m",
		"-v", codePath+":/tmp/code.sans:ro",
		imageName,
		"sh", "-c", "sans build /tmp/code.sans -o /home/runner/code && /home/runner/code",
	)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()

	result := RunResult{
		Stdout: truncate(stdout.String(), 64*1024),
		Stderr: truncate(stderr.String(), 64*1024),
	}

	if ctx.Err() == context.DeadlineExceeded {
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

	result.CompileSuccess = result.ExitCode == 0 || !bytes.Contains(stderr.Bytes(), []byte("error:"))
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
