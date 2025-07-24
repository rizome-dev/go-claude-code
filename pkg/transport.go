package pkg

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	maxBufferSize = 1024 * 1024      // 1MB
	maxStderrSize = 10 * 1024 * 1024 // 10MB
	stderrTimeout = 10 * time.Second
)

type transport struct {
	cmd         *exec.Cmd
	stdin       io.WriteCloser
	stdout      io.ReadCloser
	stderr      io.ReadCloser
	parser      *messageParser
	stderrBuf   *bytes.Buffer
	messages    chan Message
	errors      chan error
	done        chan struct{}
	closeOnce   sync.Once
	requestID   atomic.Int64
	controlResp map[string]chan *ControlResponse
	controlMu   sync.Mutex
	isStreaming bool
	mu          sync.Mutex
}

func findCLI() (string, error) {
	// First try to find 'claude' (matching Python SDK)
	cliPath, err := exec.LookPath("claude")
	if err == nil {
		return cliPath, nil
	}

	// Fall back to 'claude-code' for compatibility
	cliPath, err = exec.LookPath("claude-code")
	if err == nil {
		return cliPath, nil
	}

	searchPaths := []string{
		filepath.Join(os.Getenv("HOME"), ".npm-global", "bin", "claude"),
		filepath.Join(os.Getenv("HOME"), ".npm", "bin", "claude"),
		"/usr/local/bin/claude",
		"/opt/homebrew/bin/claude",
		// Also check claude-code paths for compatibility
		filepath.Join(os.Getenv("HOME"), ".npm-global", "bin", "claude-code"),
		filepath.Join(os.Getenv("HOME"), ".npm", "bin", "claude-code"),
		"/usr/local/bin/claude-code",
		"/opt/homebrew/bin/claude-code",
	}

	for _, path := range searchPaths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", NewCLINotFoundError(searchPaths)
}

func newTransport(ctx context.Context, options *ClaudeCodeOptions, streaming bool) (*transport, error) {
	cliPath, err := findCLI()
	if err != nil {
		return nil, err
	}

	// Build command args matching Python SDK
	args := []string{"--output-format", "stream-json", "--verbose"}
	
	// Add options as individual flags (matching Python SDK)
	if options.Model != "" {
		args = append(args, "--model", options.Model)
	}
	if options.MaxTokens > 0 {
		args = append(args, "--max-tokens", fmt.Sprintf("%d", options.MaxTokens))
	}
	if options.MaxThinkingTokens > 0 {
		args = append(args, "--max-thinking-tokens", fmt.Sprintf("%d", options.MaxThinkingTokens))
	}
	if options.SystemPrompt != "" {
		args = append(args, "--system-prompt", options.SystemPrompt)
	}
	if options.AppendSystemPrompt != "" {
		args = append(args, "--append-system-prompt", options.AppendSystemPrompt)
	}
	if len(options.AllowedTools) > 0 {
		args = append(args, "--allowed-tools", strings.Join(options.AllowedTools, ","))
	}
	if len(options.DisallowedTools) > 0 {
		args = append(args, "--disallowed-tools", strings.Join(options.DisallowedTools, ","))
	}
	if options.PermissionMode != "" {
		args = append(args, "--permission-mode", string(options.PermissionMode))
	}
	if options.ContinueConversation {
		args = append(args, "--continue-conversation")
	}
	if options.Resume != "" {
		args = append(args, "--resume", options.Resume)
	}
	if options.MaxTurns > 0 {
		args = append(args, "--max-turns", fmt.Sprintf("%d", options.MaxTurns))
	}
	
	// Add streaming-specific flags
	if streaming {
		args = append(args, "--input-format", "stream-json")
	}

	cmd := exec.CommandContext(ctx, cliPath, args...)
	
	env := os.Environ()
	// Set environment variable to match Python SDK
	env = append(env, "CLAUDE_CODE_ENTRYPOINT=sdk-go")
	cmd.Env = env

	if options.Cwd != "" {
		cmd.Dir = options.Cwd
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, NewCLIConnectionError("Failed to create stdin pipe", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, NewCLIConnectionError("Failed to create stdout pipe", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, NewCLIConnectionError("Failed to create stderr pipe", err)
	}

	t := &transport{
		cmd:         cmd,
		stdin:       stdin,
		stdout:      stdout,
		stderr:      stderr,
		parser:      newMessageParser(),
		stderrBuf:   &bytes.Buffer{},
		messages:    make(chan Message, 100),
		errors:      make(chan error, 10),
		done:        make(chan struct{}),
		controlResp: make(map[string]chan *ControlResponse),
		isStreaming: streaming,
	}

	if err := cmd.Start(); err != nil {
		return nil, NewCLIConnectionError("Failed to start Claude Code CLI", err)
	}

	go t.readStderr()
	go t.readMessages()

	return t, nil
}

// newTransportForQuery creates a transport specifically for the Query function
// This matches Python's query() behavior with close_stdin_after_prompt=True
func newTransportForQuery(ctx context.Context, options *ClaudeCodeOptions, prompt string) (*transport, error) {
	cliPath, err := findCLI()
	if err != nil {
		return nil, err
	}

	// Build command args matching Python SDK query mode
	args := []string{"--output-format", "stream-json", "--verbose"}
	
	// Add the prompt using --print flag (Python string mode)
	args = append(args, "--print", prompt)
	
	// Add options as individual flags (matching Python SDK)
	if options.Model != "" {
		args = append(args, "--model", options.Model)
	}
	if options.MaxTokens > 0 {
		args = append(args, "--max-tokens", fmt.Sprintf("%d", options.MaxTokens))
	}
	if options.MaxThinkingTokens > 0 {
		args = append(args, "--max-thinking-tokens", fmt.Sprintf("%d", options.MaxThinkingTokens))
	}
	if options.SystemPrompt != "" {
		args = append(args, "--system-prompt", options.SystemPrompt)
	}
	if options.AppendSystemPrompt != "" {
		args = append(args, "--append-system-prompt", options.AppendSystemPrompt)
	}
	if len(options.AllowedTools) > 0 {
		args = append(args, "--allowed-tools", strings.Join(options.AllowedTools, ","))
	}
	if len(options.DisallowedTools) > 0 {
		args = append(args, "--disallowed-tools", strings.Join(options.DisallowedTools, ","))
	}
	if options.PermissionMode != "" {
		args = append(args, "--permission-mode", string(options.PermissionMode))
	}
	if options.ContinueConversation {
		args = append(args, "--continue-conversation")
	}
	if options.Resume != "" {
		args = append(args, "--resume", options.Resume)
	}
	if options.MaxTurns > 0 {
		args = append(args, "--max-turns", fmt.Sprintf("%d", options.MaxTurns))
	}

	cmd := exec.CommandContext(ctx, cliPath, args...)
	
	env := os.Environ()
	// Set environment variable to match Python SDK query mode
	env = append(env, "CLAUDE_CODE_ENTRYPOINT=sdk-go-query")
	cmd.Env = env

	if options.Cwd != "" {
		cmd.Dir = options.Cwd
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, NewCLIConnectionError("Failed to create stdin pipe", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, NewCLIConnectionError("Failed to create stdout pipe", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, NewCLIConnectionError("Failed to create stderr pipe", err)
	}

	t := &transport{
		cmd:         cmd,
		stdin:       stdin,
		stdout:      stdout,
		stderr:      stderr,
		parser:      newMessageParser(),
		stderrBuf:   &bytes.Buffer{},
		messages:    make(chan Message, 100),
		errors:      make(chan error, 10),
		done:        make(chan struct{}),
		controlResp: make(map[string]chan *ControlResponse),
		isStreaming: false,
	}

	if err := cmd.Start(); err != nil {
		return nil, NewCLIConnectionError("Failed to start Claude Code CLI", err)
	}

	go t.readStderr()
	go t.readMessages()

	return t, nil
}

func (t *transport) sendMessage(ctx context.Context, message Message, parentToolUseID, sessionID string) error {
	input := InputMessage{
		Type:            "user",
		Message:         message,
		ParentToolUseID: parentToolUseID,
		SessionID:       sessionID,
	}

	data, err := json.Marshal(input)
	if err != nil {
		return err
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if _, err := t.stdin.Write(data); err != nil {
		return NewCLIConnectionError("Failed to send message", err)
	}

	if _, err := t.stdin.Write([]byte("\n")); err != nil {
		return NewCLIConnectionError("Failed to send newline", err)
	}

	return nil
}

func (t *transport) sendInterrupt(ctx context.Context) error {
	requestID := fmt.Sprintf("req_%d_%d", t.requestID.Add(1), time.Now().UnixNano())
	
	request := ControlRequest{
		Type:      "control_request",
		RequestID: requestID,
		Request: struct {
			Subtype ControlRequestType `json:"subtype"`
		}{
			Subtype: ControlRequestTypeInterrupt,
		},
	}

	data, err := json.Marshal(request)
	if err != nil {
		return err
	}

	respChan := make(chan *ControlResponse, 1)
	t.controlMu.Lock()
	t.controlResp[requestID] = respChan
	t.controlMu.Unlock()

	defer func() {
		t.controlMu.Lock()
		delete(t.controlResp, requestID)
		t.controlMu.Unlock()
	}()

	t.mu.Lock()
	if _, err := t.stdin.Write(data); err != nil {
		t.mu.Unlock()
		return NewCLIConnectionError("Failed to send interrupt", err)
	}
	if _, err := t.stdin.Write([]byte("\n")); err != nil {
		t.mu.Unlock()
		return NewCLIConnectionError("Failed to send newline", err)
	}
	t.mu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case resp := <-respChan:
		if !resp.Response.Success {
			return fmt.Errorf("interrupt failed: %s", resp.Response.Error)
		}
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("interrupt request timeout")
	}
}

func (t *transport) closeStdin() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	if t.stdin != nil {
		return t.stdin.Close()
	}
	return nil
}

func (t *transport) close() error {
	var finalErr error
	
	t.closeOnce.Do(func() {
		// First, signal done to stop goroutines
		close(t.done)
		
		// Close stdin to signal EOF to the process
		if t.stdin != nil {
			t.stdin.Close()
		}
		
		// Kill the process
		if t.cmd.Process != nil {
			t.cmd.Process.Kill()
		}
		
		// Close stdout and stderr to unblock readers
		if t.stdout != nil {
			t.stdout.Close()
		}
		if t.stderr != nil {
			t.stderr.Close()
		}
		
		// Wait for process to exit
		if t.cmd.Process != nil {
			t.cmd.Wait()
		}
		
		// Give goroutines a moment to finish
		time.Sleep(10 * time.Millisecond)
		
		// Finally, close the channels
		close(t.messages)
		close(t.errors)
	})

	return finalErr
}

func (t *transport) readMessages() {
	scanner := bufio.NewScanner(t.stdout)
	scanner.Buffer(make([]byte, maxBufferSize), maxBufferSize)

	for scanner.Scan() {
		select {
		case <-t.done:
			return
		default:
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		if t.parser.isControlResponse(line) {
			resp, err := t.parser.parseControlResponse(line)
			if err != nil {
				select {
				case t.errors <- err:
				case <-t.done:
					return
				}
				continue
			}

			t.controlMu.Lock()
			if ch, ok := t.controlResp[resp.RequestID]; ok {
				ch <- resp
			}
			t.controlMu.Unlock()
			continue
		}

		streamMsg, err := t.parser.parseStreamMessage(line)
		if err != nil {
			select {
			case t.errors <- err:
			case <-t.done:
				return
			}
			continue
		}

		msg, err := t.parser.parseMessage(streamMsg.Type, streamMsg.Message)
		if err != nil {
			select {
			case t.errors <- err:
			case <-t.done:
				return
			}
			continue
		}

		if msg != nil {
			select {
			case t.messages <- msg:
			case <-t.done:
				return
			}
		}
	}

	if err := scanner.Err(); err != nil {
		select {
		case t.errors <- NewCLIConnectionError("Error reading stdout", err):
		case <-t.done:
			return
		}
	}
}

func (t *transport) readStderr() {
	reader := bufio.NewReader(t.stderr)
	buf := make([]byte, 4096)

	for {
		select {
		case <-t.done:
			return
		default:
		}

		n, err := reader.Read(buf)
		if n > 0 {
			t.mu.Lock()
			if t.stderrBuf.Len()+n <= maxStderrSize {
				t.stderrBuf.Write(buf[:n])
			}
			t.mu.Unlock()
		}

		if err != nil {
			if err != io.EOF {
				select {
				case t.errors <- NewCLIConnectionError("Error reading stderr", err):
				case <-t.done:
					return
				}
			}
			return
		}
	}
}

func (t *transport) wait() error {
	err := t.cmd.Wait()
	
	time.Sleep(100 * time.Millisecond)
	
	t.mu.Lock()
	stderr := t.stderrBuf.String()
	t.mu.Unlock()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return NewProcessError(exitErr.ExitCode(), "", stderr)
		}
		return NewCLIConnectionError("Process failed", err)
	}

	return nil
}

func (t *transport) collectStderr(timeout time.Duration) string {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	lastSize := 0
	stableCount := 0

	for {
		select {
		case <-timer.C:
			t.mu.Lock()
			result := t.stderrBuf.String()
			t.mu.Unlock()
			return result
		case <-ticker.C:
			t.mu.Lock()
			currentSize := t.stderrBuf.Len()
			t.mu.Unlock()

			if currentSize == lastSize {
				stableCount++
				if stableCount >= 3 {
					t.mu.Lock()
					result := t.stderrBuf.String()
					t.mu.Unlock()
					return result
				}
			} else {
				stableCount = 0
				lastSize = currentSize
			}
		}
	}
}