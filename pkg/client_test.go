package pkg

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Helper to create a mock CLI for testing
func setupMockCLI(t *testing.T) {
	tmpDir := t.TempDir()
	// Create both 'claude' and 'claude-code' for compatibility
	mockPaths := []string{
		filepath.Join(tmpDir, "claude"),
		filepath.Join(tmpDir, "claude-code"),
	}
	
	script := `#!/bin/sh
# Mock Claude Code CLI for testing
while IFS= read -r line; do
    # Echo back user messages as assistant responses
    if echo "$line" | grep -q '"type":"user"'; then
        content=$(echo "$line" | sed -n 's/.*"content":"\([^"]*\)".*/\1/p')
        echo '{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Reply to: '"$content"'"}]}}'
    fi
done
# Send result message when stdin closes
echo '{"type":"system","message":{"role":"system","subtype":"result","data":{"usage":{"inputTokens":10,"outputTokens":20,"backgroundTokens":0},"cost":{"inputTokenCost":0.001,"outputTokenCost":0.002,"backgroundTokenCost":0,"totalCost":0.003},"sessionId":"test-session","interruptRequested":false}}}'
`

	for _, mockPath := range mockPaths {
		if err := os.WriteFile(mockPath, []byte(script), 0755); err != nil {
			t.Fatalf("Failed to create mock CLI at %s: %v", mockPath, err)
		}
	}

	oldPath := os.Getenv("PATH")
	os.Setenv("PATH", tmpDir+":"+oldPath)
	t.Cleanup(func() {
		os.Setenv("PATH", oldPath)
	})
}

func TestNewClient(t *testing.T) {
	setupMockCLI(t)
	
	ctx := context.Background()
	
	tests := []struct {
		name    string
		options *ClaudeCodeOptions
	}{
		{
			name:    "with nil options",
			options: nil,
		},
		{
			name: "with custom options",
			options: &ClaudeCodeOptions{
				Model:     "claude-3-opus",
				MaxTokens: 1000,
				SessionID: "custom-session",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.options)
			if client == nil {
				t.Error("NewClient() returned nil")
				return
			}
			
			// Test connection
			err := client.Connect(ctx, "")
			if err != nil {
				t.Errorf("Connect() error = %v", err)
			}
			defer client.Close()
		})
	}
}

func TestClient_SendMessage(t *testing.T) {
	setupMockCLI(t)
	
	ctx := context.Background()
	client := NewClient(nil)
	
	err := client.Connect(ctx, "")
	if err != nil {
		t.Fatalf("Failed to connect client: %v", err)
	}
	defer client.Close()

	err = client.SendMessage(ctx, "Hello, Claude!")
	if err != nil {
		t.Errorf("SendMessage() error = %v", err)
	}

	// Wait for response
	select {
	case msg := <-client.Messages():
		if msg == nil {
			t.Error("Received nil message")
		}
		if msg.GetType() != "assistant" {
			t.Errorf("Expected assistant message, got %v", msg.GetType())
		}
	case err := <-client.Errors():
		t.Errorf("Received error: %v", err)
	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for message")
	}
}

func TestClient_StreamMessages(t *testing.T) {
	setupMockCLI(t)
	
	ctx := context.Background()
	client := NewClient(nil)
	
	err := client.Connect(ctx, "")
	if err != nil {
		t.Fatalf("Failed to connect client: %v", err)
	}
	defer client.Close()

	// Start streaming messages
	streamCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	
	messages := client.StreamMessages(streamCtx)

	// Send a message
	err = client.SendMessage(ctx, "Test message")
	if err != nil {
		t.Errorf("SendMessage() error = %v", err)
	}

	// Collect messages
	var collected []Message
	for msg := range messages {
		collected = append(collected, msg)
		if _, isResult := msg.(ResultMessage); isResult {
			cancel() // Stop streaming after result
		}
	}

	if len(collected) < 2 {
		t.Errorf("Expected at least 2 messages (assistant + result), got %d", len(collected))
	}

	// Verify messages are stored
	stored := client.GetMessages()
	if len(stored) != len(collected) {
		t.Errorf("Stored messages count = %d, want %d", len(stored), len(collected))
	}
}

func TestClient_WaitForResult(t *testing.T) {
	setupMockCLI(t)
	
	ctx := context.Background()
	client := NewClient(&ClaudeCodeOptions{
		SessionID: "test-wait-result",
	})
	
	err := client.Connect(ctx, "")
	if err != nil {
		t.Fatalf("Failed to connect client: %v", err)
	}
	defer client.Close()

	// Send a message
	err = client.SendMessage(ctx, "Test for result")
	if err != nil {
		t.Errorf("SendMessage() error = %v", err)
	}

	// Wait for result
	waitCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	
	result, err := client.WaitForResult(waitCtx)
	if err != nil {
		t.Errorf("WaitForResult() error = %v", err)
	}

	if result == nil {
		t.Fatal("WaitForResult() returned nil result")
	}

	if result.Data.SessionID != "test-session" {
		t.Errorf("Result SessionID = %v, want 'test-session'", result.Data.SessionID)
	}

	if result.Data.Cost.TotalCost != 0.003 {
		t.Errorf("Result TotalCost = %v, want 0.003", result.Data.Cost.TotalCost)
	}
}

func TestClient_Close(t *testing.T) {
	setupMockCLI(t)
	
	ctx := context.Background()
	client := NewClient(nil)
	
	err := client.Connect(ctx, "")
	if err != nil {
		t.Fatalf("Failed to connect client: %v", err)
	}

	// First close should succeed
	err = client.Close()
	if err != nil {
		t.Errorf("First Close() error = %v", err)
	}

	// Second close should be safe
	err = client.Close()
	if err != nil {
		t.Errorf("Second Close() error = %v", err)
	}

	// Operations after close should fail
	err = client.SendMessage(ctx, "Should fail")
	if err == nil {
		t.Error("SendMessage() after Close() should fail")
	}
}

func TestClient_IterateMessages(t *testing.T) {
	setupMockCLI(t)
	
	ctx := context.Background()
	client := NewClient(nil)
	
	err := client.Connect(ctx, "")
	if err != nil {
		t.Fatalf("Failed to connect client: %v", err)
	}
	defer client.Close()

	// Send a message
	err = client.SendMessage(ctx, "Test iteration")
	if err != nil {
		t.Errorf("SendMessage() error = %v", err)
	}

	// Use iterator
	iterCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	
	iter := client.IterateMessages(iterCtx)
	
	messageCount := 0
	for {
		msg, err := iter.Next()
		if err != nil {
			if err == context.DeadlineExceeded {
				break
			}
			t.Errorf("Iterator.Next() error = %v", err)
			break
		}
		
		messageCount++
		
		if _, isResult := msg.(ResultMessage); isResult {
			break
		}
	}

	if messageCount < 2 {
		t.Errorf("Expected at least 2 messages, got %d", messageCount)
	}
}

func TestClient_ContextCancellation(t *testing.T) {
	setupMockCLI(t)
	
	ctx, cancel := context.WithCancel(context.Background())
	client := NewClient(nil)
	
	err := client.Connect(ctx, "")
	if err != nil {
		t.Fatalf("Failed to connect client: %v", err)
	}
	defer client.Close()

	// Start streaming
	messages := client.StreamMessages(ctx)

	// Cancel context
	cancel()

	// Stream should close
	timeout := time.After(1 * time.Second)
	for {
		select {
		case _, ok := <-messages:
			if !ok {
				// Channel closed as expected
				return
			}
		case <-timeout:
			t.Error("Stream did not close after context cancellation")
			return
		}
	}
}

func TestClient_GetMessages(t *testing.T) {
	setupMockCLI(t)
	
	ctx := context.Background()
	client := NewClient(nil)
	
	err := client.Connect(ctx, "")
	if err != nil {
		t.Fatalf("Failed to connect client: %v", err)
	}
	defer client.Close()

	// Initially should be empty
	messages := client.GetMessages()
	if len(messages) != 0 {
		t.Errorf("Initial GetMessages() length = %d, want 0", len(messages))
	}

	// Send a message and wait for response
	err = client.SendMessage(ctx, "Test message")
	if err != nil {
		t.Errorf("SendMessage() error = %v", err)
	}

	// Wait for some messages
	time.Sleep(500 * time.Millisecond)

	// Should have messages now
	messages = client.GetMessages()
	if len(messages) == 0 {
		t.Error("GetMessages() returned empty after sending message")
	}

	// Verify returned slice is a copy
	originalLen := len(messages)
	messages = append(messages, UserMessage{Role: MessageRoleUser, Content: "Extra"})
	
	newMessages := client.GetMessages()
	if len(newMessages) != originalLen {
		t.Error("GetMessages() should return a copy of messages")
	}
}

func TestClient_Connect(t *testing.T) {
	setupMockCLI(t)
	
	ctx := context.Background()
	
	t.Run("connect without prompt", func(t *testing.T) {
		client := NewClient(nil)
		
		err := client.Connect(ctx, "")
		if err != nil {
			t.Errorf("Connect() error = %v", err)
		}
		defer client.Close()
		
		// Should be able to send messages after connect
		err = client.SendMessage(ctx, "Test")
		if err != nil {
			t.Errorf("SendMessage() after Connect() error = %v", err)
		}
	})
	
	t.Run("connect with prompt", func(t *testing.T) {
		client := NewClient(nil)
		
		err := client.Connect(ctx, "Initial prompt")
		if err != nil {
			t.Errorf("Connect() with prompt error = %v", err)
		}
		defer client.Close()
		
		// Should receive response to initial prompt
		select {
		case msg := <-client.Messages():
			if msg == nil {
				t.Error("Expected message from initial prompt")
			}
		case <-time.After(2 * time.Second):
			t.Error("Timeout waiting for response to initial prompt")
		}
	})
	
	t.Run("double connect", func(t *testing.T) {
		client := NewClient(nil)
		
		err := client.Connect(ctx, "")
		if err != nil {
			t.Errorf("First Connect() error = %v", err)
		}
		defer client.Close()
		
		// Second connect should fail
		err = client.Connect(ctx, "")
		if err == nil {
			t.Error("Second Connect() should fail")
		}
	})
}

func TestClient_ReceiveResponse(t *testing.T) {
	setupMockCLI(t)
	
	ctx := context.Background()
	client := NewClient(nil)
	
	err := client.Connect(ctx, "")
	if err != nil {
		t.Fatalf("Failed to connect client: %v", err)
	}
	defer client.Close()
	
	// Send a message
	err = client.SendMessage(ctx, "Test for response")
	if err != nil {
		t.Errorf("SendMessage() error = %v", err)
	}
	
	// Receive response
	respCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	
	responses := client.ReceiveResponse(respCtx)
	
	var messageCount int
	var gotResult bool
	
	for msg := range responses {
		messageCount++
		if _, isResult := msg.(ResultMessage); isResult {
			gotResult = true
		}
	}
	
	if messageCount < 2 {
		t.Errorf("Expected at least 2 messages, got %d", messageCount)
	}
	
	if !gotResult {
		t.Error("ReceiveResponse() should include ResultMessage")
	}
}