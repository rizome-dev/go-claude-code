package pkg

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// MockCLIResponse represents a response from the mock CLI
type MockCLIResponse struct {
	Type    string          `json:"type"`
	Message json.RawMessage `json:"message"`
}

// CreateMockCLI creates a mock CLI that responds with the given messages
func CreateMockCLI(t *testing.T, responses []interface{}) {
	tmpDir := t.TempDir()
	
	// Create response file with properly formatted JSON
	responsesFile := filepath.Join(tmpDir, "responses.json")
	var jsonLines []string
	
	for _, resp := range responses {
		data, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("Failed to marshal response: %v", err)
		}
		jsonLines = append(jsonLines, string(data))
	}
	
	responseContent := ""
	for _, line := range jsonLines {
		responseContent += line + "\n"
	}
	
	if err := os.WriteFile(responsesFile, []byte(responseContent), 0644); err != nil {
		t.Fatalf("Failed to write responses file: %v", err)
	}
	
	// Create mock CLI script
	script := fmt.Sprintf(`#!/bin/sh
# Mock Claude CLI for testing
# Read responses from file and output them
cat %s
`, responsesFile)
	
	// Create both 'claude' and 'claude-code' executables
	for _, name := range []string{"claude", "claude-code"} {
		mockPath := filepath.Join(tmpDir, name)
		if err := os.WriteFile(mockPath, []byte(script), 0755); err != nil {
			t.Fatalf("Failed to create mock %s: %v", name, err)
		}
	}
	
	// Update PATH
	oldPath := os.Getenv("PATH")
	os.Setenv("PATH", tmpDir+":"+oldPath)
	t.Cleanup(func() {
		os.Setenv("PATH", oldPath)
	})
}

// CreateAssistantMessage creates a properly formatted assistant message
func CreateAssistantMessage(content []ContentBlock) interface{} {
	msg := AssistantMessage{
		Role:    MessageRoleAssistant,
		Content: content,
	}
	
	data, _ := json.Marshal(msg)
	return MockCLIResponse{
		Type:    "assistant",
		Message: data,
	}
}

// CreateResultMessage creates a properly formatted result message
func CreateResultMessage(sessionID string, inputTokens, outputTokens int, totalCost float64) interface{} {
	msg := ResultMessage{
		Role: MessageRoleSystem,
		Data: ResultMessageData{
			Usage: ResultUsage{
				InputTokens:  inputTokens,
				OutputTokens: outputTokens,
			},
			Cost: ResultCost{
				InputTokenCost:  totalCost * 0.3,
				OutputTokenCost: totalCost * 0.7,
				TotalCost:       totalCost,
			},
			SessionID: sessionID,
		},
	}
	
	data, _ := json.Marshal(msg)
	return MockCLIResponse{
		Type:    "system",
		Message: data,
	}
}

// CreateSystemMessage creates a properly formatted system message
func CreateSystemMessage(subtype SystemMessageSubtype, data interface{}) interface{} {
	msg := SystemMessage{
		Role:    MessageRoleSystem,
		Subtype: subtype,
		Data:    data,
	}
	
	msgData, _ := json.Marshal(msg)
	return MockCLIResponse{
		Type:    "system",
		Message: msgData,
	}
}