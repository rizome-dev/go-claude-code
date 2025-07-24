package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/rizome-dev/go-claude-code/pkg"
)

func main() {
	ctx := context.Background()

	// Advanced configuration with MCP servers
	options := &pkg.ClaudeCodeOptions{
		Model:               "claude-3-opus-20240229",
		MaxTokens:           4000,
		MaxBackgroundTokens: 10000,
		MaxCostUSD:          1.0,
		Temperature:         0.5,
		CustomInstructions:  "You are a helpful coding assistant. Always explain your code clearly.",
		Mode:                pkg.PermissionModeAcceptEdits,
		OnlyTools:           []string{"bash", "write_file", "read_file"},
		McpServers: map[string]pkg.MCPServerConfig{
			"filesystem": {
				Type:    pkg.MCPServerTypeStdio,
				Command: "mcp-server-filesystem",
				Args:    []string{"/tmp/workspace"},
				Env: map[string]string{
					"READ_ONLY": "false",
				},
			},
			"github": {
				Type:   pkg.MCPServerTypeHTTP,
				URL:    "https://api.github.com/mcp",
				APIKey: "your-api-key",
				Headers: map[string]string{
					"X-Custom-Header": "value",
				},
			},
		},
		Cwd:       "/tmp/workspace",
		SessionID: "advanced-example",
	}

	// Create client with advanced options
	client, err := pkg.NewClient(ctx, options)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Send a complex request
	prompt := `Create a simple Go web server that:
1. Listens on port 8080
2. Has a /health endpoint that returns {"status": "ok"}
3. Has a /time endpoint that returns the current time
4. Logs all requests to stdout`

	if err := client.SendMessage(ctx, prompt); err != nil {
		log.Fatalf("Failed to send message: %v", err)
	}

	// Process messages with timeout
	processCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	fmt.Println("Processing request...")
	
	for {
		select {
		case <-processCtx.Done():
			fmt.Println("Processing timeout reached")
			return
			
		case err := <-client.Errors():
			log.Printf("Error: %v", err)
			
		case msg := <-client.Messages():
			if msg == nil {
				continue
			}
			
			switch m := msg.(type) {
			case *pkg.AssistantMessage:
				processAssistantMessage(m)
				
			case pkg.SystemMessage:
				processSystemMessage(m)
				
			case pkg.ResultMessage:
				fmt.Println("\n=== Final Result ===")
				fmt.Printf("Session ID: %s\n", m.Data.SessionID)
				fmt.Printf("Total tokens: %d\n", 
					m.Data.Usage.InputTokens + m.Data.Usage.OutputTokens)
				fmt.Printf("Total cost: $%.4f\n", m.Data.Cost.TotalCost)
				
				if m.Data.InterruptRequested {
					fmt.Println("Note: Response was interrupted")
				}
				return
			}
		}
	}
}

func processAssistantMessage(msg *pkg.AssistantMessage) {
	for _, block := range msg.Content {
		switch b := block.(type) {
		case pkg.TextBlock:
			fmt.Printf("Claude: %s\n", b.Text)
			
		case pkg.ToolUseBlock:
			fmt.Printf("\n[Tool Use] %s (ID: %s)\n", b.Name, b.ID)
			if len(b.Input) > 0 {
				fmt.Println("Input:")
				for k, v := range b.Input {
					fmt.Printf("  %s: %v\n", k, v)
				}
			}
			
		case pkg.ToolResultBlock:
			fmt.Printf("\n[Tool Result for %s]\n", b.ToolUseID)
			if b.IsError {
				fmt.Printf("Error: %v\n", b.Content)
			} else {
				fmt.Printf("Success: %v\n", b.Content)
			}
		}
	}
}

func processSystemMessage(msg pkg.SystemMessage) {
	switch msg.Subtype {
	case pkg.SystemMessageSubtypeUsage:
		// Periodic usage updates
		if usage, ok := msg.Data.(map[string]interface{}); ok {
			fmt.Printf("[Usage Update] Tokens: %.0f\n", usage["tokens"])
		}
		
	case pkg.SystemMessageSubtypeThinking:
		fmt.Println("[Claude is thinking...]")
		
	case pkg.SystemMessageSubtypeMCPServerLog:
		if log, ok := msg.Data.(map[string]interface{}); ok {
			fmt.Printf("[MCP Server Log] %v: %v\n", log["server"], log["message"])
		}
		
	case pkg.SystemMessageSubtypeFile:
		if file, ok := msg.Data.(map[string]interface{}); ok {
			fmt.Printf("[File Operation] %v: %v\n", file["operation"], file["path"])
		}
	}
}