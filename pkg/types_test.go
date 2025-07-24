package pkg

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestMCPServerConfig_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected MCPServerConfig
		wantErr  bool
	}{
		{
			name: "stdio server",
			input: `{
				"type": "stdio",
				"command": "node",
				"args": ["server.js"],
				"env": {"NODE_ENV": "production"}
			}`,
			expected: MCPServerConfig{
				Type:    MCPServerTypeStdio,
				Command: "node",
				Args:    []string{"server.js"},
				Env:     map[string]string{"NODE_ENV": "production"},
			},
		},
		{
			name: "sse server",
			input: `{
				"type": "sse",
				"url": "https://api.example.com/sse",
				"apiKey": "secret123",
				"headers": {"X-Custom": "value"}
			}`,
			expected: MCPServerConfig{
				Type:    MCPServerTypeSSE,
				URL:     "https://api.example.com/sse",
				APIKey:  "secret123",
				Headers: map[string]string{"X-Custom": "value"},
			},
		},
		{
			name: "legacy format (no type)",
			input: `{
				"command": "python",
				"args": ["-m", "server"],
				"env": {"PYTHONPATH": "/app"}
			}`,
			expected: MCPServerConfig{
				Command: "python",
				Args:    []string{"-m", "server"},
				Env:     map[string]string{"PYTHONPATH": "/app"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got MCPServerConfig
			err := json.Unmarshal([]byte(tt.input), &got)
			
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("UnmarshalJSON() = %+v, want %+v", got, tt.expected)
			}
		})
	}
}

func TestAssistantMessage_UnmarshalJSON(t *testing.T) {
	input := `{
		"role": "assistant",
		"content": [
			{
				"type": "text",
				"text": "Hello, world!"
			},
			{
				"type": "tool_use",
				"id": "tool123",
				"name": "calculator",
				"input": {"a": 1, "b": 2}
			},
			{
				"type": "tool_result",
				"tool_use_id": "tool123",
				"content": "3",
				"is_error": false
			}
		]
	}`

	var msg AssistantMessage
	err := json.Unmarshal([]byte(input), &msg)
	if err != nil {
		t.Fatalf("UnmarshalJSON() error = %v", err)
	}

	if msg.Role != MessageRoleAssistant {
		t.Errorf("Role = %v, want %v", msg.Role, MessageRoleAssistant)
	}

	if len(msg.Content) != 3 {
		t.Fatalf("Content length = %d, want 3", len(msg.Content))
	}

	textBlock, ok := msg.Content[0].(TextBlock)
	if !ok {
		t.Errorf("Content[0] type = %T, want TextBlock", msg.Content[0])
	} else if textBlock.Text != "Hello, world!" {
		t.Errorf("TextBlock.Text = %v, want 'Hello, world!'", textBlock.Text)
	}

	toolUseBlock, ok := msg.Content[1].(ToolUseBlock)
	if !ok {
		t.Errorf("Content[1] type = %T, want ToolUseBlock", msg.Content[1])
	} else {
		if toolUseBlock.ID != "tool123" {
			t.Errorf("ToolUseBlock.ID = %v, want 'tool123'", toolUseBlock.ID)
		}
		if toolUseBlock.Name != "calculator" {
			t.Errorf("ToolUseBlock.Name = %v, want 'calculator'", toolUseBlock.Name)
		}
	}

	toolResultBlock, ok := msg.Content[2].(ToolResultBlock)
	if !ok {
		t.Errorf("Content[2] type = %T, want ToolResultBlock", msg.Content[2])
	} else {
		if toolResultBlock.ToolUseID != "tool123" {
			t.Errorf("ToolResultBlock.ToolUseID = %v, want 'tool123'", toolResultBlock.ToolUseID)
		}
		if toolResultBlock.IsError {
			t.Errorf("ToolResultBlock.IsError = %v, want false", toolResultBlock.IsError)
		}
	}
}

func TestMessageInterfaces(t *testing.T) {
	tests := []struct {
		name     string
		message  Message
		wantRole MessageRole
		wantType string
	}{
		{
			name:     "UserMessage",
			message:  UserMessage{Role: MessageRoleUser, Content: "test"},
			wantRole: MessageRoleUser,
			wantType: "user",
		},
		{
			name:     "AssistantMessage",
			message:  &AssistantMessage{Role: MessageRoleAssistant},
			wantRole: MessageRoleAssistant,
			wantType: "assistant",
		},
		{
			name:     "SystemMessage",
			message:  SystemMessage{Role: MessageRoleSystem},
			wantRole: MessageRoleSystem,
			wantType: "system",
		},
		{
			name:     "ResultMessage",
			message:  ResultMessage{},
			wantRole: MessageRoleSystem,
			wantType: "result",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.message.GetRole(); got != tt.wantRole {
				t.Errorf("GetRole() = %v, want %v", got, tt.wantRole)
			}
			if got := tt.message.GetType(); got != tt.wantType {
				t.Errorf("GetType() = %v, want %v", got, tt.wantType)
			}
		})
	}
}

func TestContentBlockInterfaces(t *testing.T) {
	tests := []struct {
		name     string
		block    ContentBlock
		wantType string
	}{
		{
			name:     "TextBlock",
			block:    TextBlock{Type: "text", Text: "hello"},
			wantType: "text",
		},
		{
			name:     "ToolUseBlock",
			block:    ToolUseBlock{Type: "tool_use"},
			wantType: "tool_use",
		},
		{
			name:     "ToolResultBlock",
			block:    ToolResultBlock{Type: "tool_result"},
			wantType: "tool_result",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.block.GetType(); got != tt.wantType {
				t.Errorf("GetType() = %v, want %v", got, tt.wantType)
			}
		})
	}
}

func TestClaudeCodeOptions_JSON(t *testing.T) {
	options := ClaudeCodeOptions{
		ApiKeyName:  "test-key",
		Model:       "claude-3-opus",
		MaxTokens:   1000,
		Temperature: 0.7,
		Mode:        PermissionModeAcceptEdits,
		OnlyTools:   []string{"bash", "python"},
		Cwd:         "/tmp/test",
		SessionID:   "session123",
	}

	data, err := json.Marshal(options)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var decoded ClaudeCodeOptions
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if !reflect.DeepEqual(options, decoded) {
		t.Errorf("Round trip failed:\ngot  = %+v\nwant = %+v", decoded, options)
	}
}