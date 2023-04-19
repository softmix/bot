package tools

import (
	"bytes"
	"os/exec"
)

func init() {
	RegisterTool(TerminalTool{})
}

type TerminalTool struct{}

func (tool TerminalTool) Name() string {
	return "Terminal"
}

func (tool TerminalTool) Description() string {
	return "Executes commands in a terminal. Input should be valid commands, and the output will be any output from running that command."
}

func (tool TerminalTool) Run(input string) (string, error) {
	cmd := exec.Command("sh", "-c", input)
	var stdout, stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout

	err := cmd.Run()
	if err != nil {
		return stderr.String(), err
	}

	return stdout.String(), nil
}
