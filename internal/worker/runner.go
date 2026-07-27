package worker

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

type RunResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
	Error    string
}

func RunCommand(command string) RunResult {
	return RunCommandWithTimeout(command, 0)
}

func RunCommandWithTimeout(command string, timeoutSec int) RunResult {
	ctx := context.Background()
	var cancel context.CancelFunc
	if timeoutSec > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", command)

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()

	result := RunResult{
		Stdout: stdoutBuf.String(),
		Stderr: stderrBuf.String(),
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			result.ExitCode = -1
			result.Error = fmt.Sprintf("timeout after %ds", timeoutSec)
		} else if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
			result.Error = fmt.Sprintf("exit code %d", result.ExitCode)
		} else {
			result.ExitCode = -1
			result.Error = err.Error()
		}
	}

	return result
}

