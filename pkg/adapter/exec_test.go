package adapter

import (
	"context"
	"testing"
)

// MockExecutor returns canned output for testing adapters that shell out.
type MockExecutor struct {
	Output []byte
	Err    error
	// Calls records each invocation for assertion.
	Calls []MockCall
}

type MockCall struct {
	Name string
	Args []string
}

func (m *MockExecutor) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	m.Calls = append(m.Calls, MockCall{Name: name, Args: args})
	return m.Output, m.Err
}

func TestShellExecutor(t *testing.T) {
	e := ShellExecutor{}
	out, err := e.Run(context.Background(), "echo", "hello")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if string(out) != "hello\n" {
		t.Errorf("output = %q, want %q", out, "hello\n")
	}
}

func TestMockExecutor(t *testing.T) {
	m := &MockExecutor{Output: []byte(`{"ok":true}`)}
	out, err := m.Run(context.Background(), "some-cli", "--flag", "value")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if string(out) != `{"ok":true}` {
		t.Errorf("output = %q", out)
	}
	if len(m.Calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(m.Calls))
	}
	if m.Calls[0].Name != "some-cli" {
		t.Errorf("call name = %q, want some-cli", m.Calls[0].Name)
	}
}
