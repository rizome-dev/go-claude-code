package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/rizome-dev/go-claude-code/pkg"
)

func main() {
	ctx := context.Background()

	// Create a client for interactive conversation
	client, err := pkg.NewClient(ctx, &pkg.ClaudeCodeOptions{
		Model:     "claude-3-opus-20240229",
		MaxTokens: 2000,
		SessionID: "interactive-example",
	})
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	fmt.Println("Claude Code Interactive Client")
	fmt.Println("Type 'quit' to exit, 'interrupt' to interrupt Claude")
	fmt.Println("---")

	// Start a goroutine to handle messages
	go handleMessages(client)

	// Read user input
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		
		switch input {
		case "quit":
			fmt.Println("Goodbye!")
			return
		case "interrupt":
			if err := client.SendInterrupt(ctx); err != nil {
				fmt.Printf("Failed to send interrupt: %v\n", err)
			} else {
				fmt.Println("Interrupt sent.")
			}
		case "":
			continue
		default:
			if err := client.SendMessage(ctx, input); err != nil {
				fmt.Printf("Failed to send message: %v\n", err)
			}
		}
	}
}

func handleMessages(client *pkg.Client) {
	ctx := context.Background()
	
	for msg := range client.StreamMessages(ctx) {
		switch m := msg.(type) {
		case *pkg.AssistantMessage:
			fmt.Print("\nClaude: ")
			for _, block := range m.Content {
				switch b := block.(type) {
				case pkg.TextBlock:
					fmt.Print(b.Text)
				case pkg.ToolUseBlock:
					fmt.Printf("\n[Using tool: %s]\n", b.Name)
				case pkg.ToolResultBlock:
					fmt.Printf("\n[Tool result: %v]\n", b.Content)
				}
			}
			fmt.Println()
			
		case pkg.SystemMessage:
			switch m.Subtype {
			case pkg.SystemMessageSubtypeThinking:
				fmt.Println("\n[Claude is thinking...]")
			case pkg.SystemMessageSubtypeInterrupted:
				fmt.Println("\n[Response interrupted]")
			}
			
		case pkg.ResultMessage:
			fmt.Printf("\n---\nSession: %s\n", m.Data.SessionID)
			fmt.Printf("Tokens: %d in, %d out\n", 
				m.Data.Usage.InputTokens, 
				m.Data.Usage.OutputTokens)
			fmt.Printf("Cost: $%.4f\n", m.Data.Cost.TotalCost)
			fmt.Println("---")
		}
	}
}