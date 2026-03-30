package llm

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
)

// StdinBackend pipes prompts to a long-running process via stdin/stdout.
// The process stays alive between calls. Protocol: write JSON line in, read JSON line out.
type StdinBackend struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Scanner
	mu     sync.Mutex
}

// stdinRequest is what we write to the process.
type stdinRequest struct {
	System string `json:"system"`
	Prompt string `json:"prompt"`
}

// stdinResponse is what we read back.
type stdinResponse struct {
	Response string `json:"response"`
	Error    string `json:"error,omitempty"`
}

// NewStdinBackend starts a process and communicates via stdin/stdout.
// The command is split by spaces (e.g. "claude --dangerously-skip-permissions").
func NewStdinBackend(command string) (*StdinBackend, error) {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty command")
	}

	cmd := exec.Command(parts[0], parts[1:]...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start process: %w", err)
	}

	return &StdinBackend{
		cmd:    cmd,
		stdin:  stdin,
		stdout: bufio.NewScanner(stdout),
	}, nil
}

func (s *StdinBackend) Complete(ctx context.Context, system, prompt string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	req := stdinRequest{System: system, Prompt: prompt}
	data, _ := json.Marshal(req)
	data = append(data, '\n')

	if _, err := s.stdin.Write(data); err != nil {
		return "", fmt.Errorf("write to stdin: %w", err)
	}

	// Read response line
	if !s.stdout.Scan() {
		if err := s.stdout.Err(); err != nil {
			return "", fmt.Errorf("read stdout: %w", err)
		}
		return "", fmt.Errorf("process closed stdout")
	}

	var resp stdinResponse
	if err := json.Unmarshal(s.stdout.Bytes(), &resp); err != nil {
		// If not JSON, treat the whole line as the response
		return s.stdout.Text(), nil
	}

	if resp.Error != "" {
		return "", fmt.Errorf("process error: %s", resp.Error)
	}

	return resp.Response, nil
}

func (s *StdinBackend) Close() error {
	s.stdin.Close()
	return s.cmd.Wait()
}
