package pkg

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// setupQueryMockCLI creates a mock CLI for query testing
func setupQueryMockCLI(t *testing.T, behavior string) {
	tmpDir := t.TempDir()
	// Create both 'claude' and 'claude-code' for compatibility
	mockPaths := []string{
		filepath.Join(tmpDir, "claude"),
		filepath.Join(tmpDir, "claude-code"),
	}
	
	var script string
	switch behavior {
	case "simple":
		script = `#!/bin/sh
# Mock CLI that parses --print flag and sends appropriate response
# The new transport uses --print flag for queries
echo '{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Response to query"}]}}'
echo '{"type":"system","message":{"role":"system","subtype":"result","data":{"usage":{"inputTokens":5,"outputTokens":3,"backgroundTokens":0},"cost":{"inputTokenCost":0.0005,"outputTokenCost":0.0006,"backgroundTokenCost":0,"totalCost":0.0011},"sessionId":"query-session","interruptRequested":false}}}'
`
	case "multi-block":
		script = `#!/bin/sh
echo '{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"First part"},{"type":"text","text":"Second part"}]}}'
echo '{"type":"system","message":{"role":"system","subtype":"result","data":{"usage":{"inputTokens":5,"outputTokens":6,"backgroundTokens":0},"cost":{"inputTokenCost":0.0005,"outputTokenCost":0.0012,"backgroundTokenCost":0,"totalCost":0.0017},"sessionId":"multi-session","interruptRequested":false}}}'
`
	case "with-tools":
		script = `#!/bin/sh
echo '{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Let me calculate that"},{"type":"tool_use","id":"calc1","name":"calculator","input":{"a":5,"b":3}},{"type":"tool_result","tool_use_id":"calc1","content":"8"}]}}'
echo '{"type":"system","message":{"role":"system","subtype":"result","data":{"usage":{"inputTokens":10,"outputTokens":15,"backgroundTokens":0},"cost":{"inputTokenCost":0.001,"outputTokenCost":0.003,"backgroundTokenCost":0,"totalCost":0.004},"sessionId":"tool-session","interruptRequested":false}}}'
`
	case "error":
		script = `#!/bin/sh
echo "Query processing error" >&2
exit 1
`
	case "timeout":
		script = `#!/bin/sh
sleep 35
`
	default:
		script = `#!/bin/sh
echo '{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Default response"}]}}'
`
	}

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

func TestQuery(t *testing.T) {
	tests := []struct {
		name         string
		mockBehavior string
		prompt       string
		options      *ClaudeCodeOptions
		wantStdout   string
		wantErr      bool
		checkResult  func(*testing.T, *QueryResult)
	}{
		{
			name:         "simple query",
			mockBehavior: "simple",
			prompt:       "Hello, Claude",
			options:      nil,
			wantStdout:   "Response to query",
			wantErr:      false,
		},
		{
			name:         "query with options",
			mockBehavior: "simple",
			prompt:       "Test with options",
			options: &ClaudeCodeOptions{
				Model:     "claude-3-opus",
				MaxTokens: 1000,
				SessionID: "custom-session",
			},
			wantStdout: "Response to query",
			wantErr:    false,
		},
		{
			name:         "multi-block response",
			mockBehavior: "multi-block",
			prompt:       "Multi-part response",
			options:      nil,
			wantStdout:   "First part\nSecond part",
			wantErr:      false,
		},
		{
			name:         "response with tools",
			mockBehavior: "with-tools",
			prompt:       "Calculate something",
			options:      nil,
			wantStdout:   "Let me calculate that",
			wantErr:      false,
			checkResult: func(t *testing.T, result *QueryResult) {
				if len(result.Messages) < 2 {
					t.Errorf("Expected at least 2 messages, got %d", len(result.Messages))
				}
			},
		},
		{
			name:         "error case",
			mockBehavior: "error",
			prompt:       "This will fail",
			options:      nil,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupQueryMockCLI(t, tt.mockBehavior)
			
			ctx := context.Background()
			result, err := Query(ctx, tt.prompt, tt.options)
			
			if (err != nil) != tt.wantErr {
				t.Errorf("Query() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if !tt.wantErr {
				if result.Stdout != tt.wantStdout {
					t.Errorf("Query() stdout = %v, want %v", result.Stdout, tt.wantStdout)
				}
				
				if result.Result == nil {
					t.Error("Query() result.Result is nil")
				}
				
				if tt.checkResult != nil {
					tt.checkResult(t, result)
				}
			}
		})
	}
}

func TestSimpleQuery(t *testing.T) {
	setupQueryMockCLI(t, "simple")
	
	ctx := context.Background()
	response, err := SimpleQuery(ctx, "Simple test")
	
	if err != nil {
		t.Errorf("SimpleQuery() error = %v", err)
	}
	
	if response != "Response to query" {
		t.Errorf("SimpleQuery() = %v, want 'Response to query'", response)
	}
}

func TestQueryWithOptions(t *testing.T) {
	// Create mock with proper responses
	CreateMockCLI(t, []interface{}{
		CreateAssistantMessage([]ContentBlock{
			TextBlock{Type: "text", Text: "Response to query"},
		}),
		CreateResultMessage("options-test", 5, 3, 0.0011),
	})
	
	ctx := context.Background()
	result, err := QueryWithOptions(ctx, "Test query", func(opts *ClaudeCodeOptions) {
		opts.Model = "claude-3-opus"
		opts.MaxThinkingTokens = 2000
		opts.Temperature = 0.7
		opts.PermissionMode = PermissionModeAcceptEdits
	})
	
	if err != nil {
		t.Errorf("QueryWithOptions() error = %v", err)
	}
	
	if result == nil {
		t.Fatal("QueryWithOptions() returned nil result")
	}
	
	if result.Stdout != "Response to query" {
		t.Errorf("QueryWithOptions() stdout = %v, want 'Response to query'", result.Stdout)
	}
}

func TestQuery_ContextCancellation(t *testing.T) {
	setupQueryMockCLI(t, "timeout")
	
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	
	_, err := Query(ctx, "This will be cancelled", nil)
	
	if err == nil {
		t.Error("Query() with cancelled context should return error")
	}
	
	if !strings.Contains(err.Error(), "context") {
		t.Errorf("Expected context error, got: %v", err)
	}
}

func TestQuery_StderrCapture(t *testing.T) {
	// Create a mock that writes to stderr
	tmpDir := t.TempDir()
	mockPath := filepath.Join(tmpDir, "claude-code")
	
	script := `#!/bin/sh
read line
echo "Warning: This is stderr" >&2
echo "Error: Another stderr line" >&2
echo '{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Response with warnings"}]}}'
echo '{"type":"system","message":{"role":"system","subtype":"result","data":{"usage":{"inputTokens":5,"outputTokens":3,"backgroundTokens":0},"cost":{"inputTokenCost":0.0005,"outputTokenCost":0.0006,"backgroundTokenCost":0,"totalCost":0.0011},"sessionId":"stderr-session","interruptRequested":false}}}'
`
	
	if err := os.WriteFile(mockPath, []byte(script), 0755); err != nil {
		t.Fatalf("Failed to create mock CLI: %v", err)
	}
	
	oldPath := os.Getenv("PATH")
	os.Setenv("PATH", tmpDir+":"+oldPath)
	defer os.Setenv("PATH", oldPath)

	ctx := context.Background()
	result, err := Query(ctx, "Test stderr", nil)
	
	if err != nil {
		t.Errorf("Query() error = %v", err)
	}
	
	if result.Stderr == "" {
		t.Error("Query() should capture stderr")
	}
	
	if !strings.Contains(result.Stderr, "Warning: This is stderr") {
		t.Errorf("Stderr should contain warning, got: %v", result.Stderr)
	}
}

func TestQuery_ResultParsing(t *testing.T) {
	setupQueryMockCLI(t, "simple")
	
	ctx := context.Background()
	result, err := Query(ctx, "Test result parsing", nil)
	
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	
	if result.Result == nil {
		t.Fatal("Query() result.Result is nil")
	}
	
	// Check usage data
	if result.Result.Data.Usage.InputTokens != 5 {
		t.Errorf("InputTokens = %d, want 5", result.Result.Data.Usage.InputTokens)
	}
	
	if result.Result.Data.Usage.OutputTokens != 3 {
		t.Errorf("OutputTokens = %d, want 3", result.Result.Data.Usage.OutputTokens)
	}
	
	// Check cost data
	if result.Result.Data.Cost.TotalCost != 0.0011 {
		t.Errorf("TotalCost = %f, want 0.0011", result.Result.Data.Cost.TotalCost)
	}
	
	// Check session ID
	if result.Result.Data.SessionID != "query-session" {
		t.Errorf("SessionID = %v, want 'query-session'", result.Result.Data.SessionID)
	}
}

func TestQuery_EmptyResponse(t *testing.T) {
	// Create a mock that sends only a result message
	tmpDir := t.TempDir()
	mockPath := filepath.Join(tmpDir, "claude-code")
	
	script := `#!/bin/sh
read line
echo '{"type":"system","message":{"role":"system","subtype":"result","data":{"usage":{"inputTokens":1,"outputTokens":0,"backgroundTokens":0},"cost":{"inputTokenCost":0.0001,"outputTokenCost":0,"backgroundTokenCost":0,"totalCost":0.0001},"sessionId":"empty-session","interruptRequested":false}}}'
`
	
	if err := os.WriteFile(mockPath, []byte(script), 0755); err != nil {
		t.Fatalf("Failed to create mock CLI: %v", err)
	}
	
	oldPath := os.Getenv("PATH")
	os.Setenv("PATH", tmpDir+":"+oldPath)
	defer os.Setenv("PATH", oldPath)

	ctx := context.Background()
	result, err := Query(ctx, "Empty response test", nil)
	
	if err != nil {
		t.Errorf("Query() error = %v", err)
	}
	
	if result.Stdout != "" {
		t.Errorf("Query() stdout = %v, want empty", result.Stdout)
	}
	
	if len(result.Messages) != 1 {
		t.Errorf("Messages length = %d, want 1", len(result.Messages))
	}
}