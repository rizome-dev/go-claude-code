package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/rizome-dev/go-claude-code/pkg"
)

func main() {
	// Example 1: Simple one-shot query
	simpleQueryExample()
	
	// Example 2: Interactive client
	interactiveClientExample()
}

func simpleQueryExample() {
	fmt.Println("=== Simple Query Example ===")
	
	ctx := context.Background()
	
	// Simple query with default options
	result, err := pkg.Query(ctx, "What is the capital of France?", nil)
	if err != nil {
		log.Printf("Query error: %v", err)
		return
	}
	
	fmt.Printf("Response: %s\n", result.Stdout)
	fmt.Printf("Cost: $%.4f\n", result.Result.Data.Cost.TotalCost)
	fmt.Println()
	
	// Query with options
	options := &pkg.ClaudeCodeOptions{
		Model:             "claude-3-opus-20240229",
		MaxThinkingTokens: 5000,
		SystemPrompt:      "You are a helpful geography assistant.",
	}
	
	result2, err := pkg.Query(ctx, "Tell me about Paris", options)
	if err != nil {
		log.Printf("Query with options error: %v", err)
		return
	}
	
	fmt.Printf("Response with options: %s\n", result2.Stdout)
	fmt.Println()
}

func interactiveClientExample() {
	fmt.Println("=== Interactive Client Example ===")
	
	ctx := context.Background()
	
	// Create client with options
	options := &pkg.ClaudeCodeOptions{
		Model:        "claude-3-opus-20240229",
		MaxTokens:    4096,
		PermissionMode: pkg.PermissionModeAcceptEdits,
	}
	
	client := pkg.NewClient(options)
	
	// Connect to Claude
	err := client.Connect(ctx, "Hello! I'm ready to help with coding tasks.")
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()
	
	// Stream messages in a goroutine
	go func() {
		messages := client.StreamMessages(ctx)
		for msg := range messages {
			switch m := msg.(type) {
			case *pkg.AssistantMessage:
				fmt.Println("Assistant:")
				for _, block := range m.Content {
					if text, ok := block.(pkg.TextBlock); ok {
						fmt.Println(text.Text)
					}
				}
			case pkg.ResultMessage:
				fmt.Printf("\n[Session complete - Cost: $%.4f]\n", m.Data.Cost.TotalCost)
			}
		}
	}()
	
	// Send a few messages
	prompts := []string{
		"Can you help me write a function to calculate fibonacci numbers?",
		"Make it recursive please",
		"Now add memoization to improve performance",
	}
	
	for _, prompt := range prompts {
		fmt.Printf("\nUser: %s\n", prompt)
		err := client.SendMessage(ctx, prompt)
		if err != nil {
			log.Printf("Failed to send message: %v", err)
			break
		}
		
		// Wait a bit between messages
		time.Sleep(2 * time.Second)
	}
	
	// Wait for final result
	result, err := client.WaitForResult(ctx)
	if err != nil {
		log.Printf("Failed to get result: %v", err)
		return
	}
	
	fmt.Printf("\nFinal token usage: %d input, %d output\n", 
		result.Data.Usage.InputTokens, 
		result.Data.Usage.OutputTokens)
}