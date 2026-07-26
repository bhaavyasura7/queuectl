package worker

import (
	"bytes"
	"fmt"
	"os/exec"
)

type RunResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
	Error    string
}

func RunCommand(command string) RunResult {
	cmd := exec.Command("sh", "-c", command)

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()

	result := RunResult{
		Stdout: stdoutBuf.String(),
		Stderr: stderrBuf.String(),
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
			result.Error = fmt.Sprintf("exit code %d", result.ExitCode)
		} else {
			result.ExitCode = -1
			result.Error = err.Error()
		}
	}

	return result
}
