package main

import (
	"context"
	"fmt"
	"log"

	"github.com/rizome-dev/go-claude-code/pkg"
)

func main() {
	ctx := context.Background()

	// Simple query - the easiest way to use the SDK
	response, err := pkg.SimpleQuery(ctx, "What is the capital of France?")
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}

	fmt.Println("Claude's response:")
	fmt.Println(response)

	fmt.Println("\n---\n")

	// Query with options
	result, err := pkg.QueryWithOptions(ctx, 
		"Write a haiku about programming in Go", 
		func(opts *pkg.ClaudeCodeOptions) {
			opts.Model = "claude-3-opus-20240229"
			opts.MaxTokens = 100
			opts.Temperature = 0.7
		})
	if err != nil {
		log.Fatalf("Query with options failed: %v", err)
	}

	fmt.Println("Claude's haiku:")
	fmt.Println(result.Stdout)
	
	if result.Result != nil {
		fmt.Printf("\nTokens used: %d input, %d output\n", 
			result.Result.Data.Usage.InputTokens,
			result.Result.Data.Usage.OutputTokens)
		fmt.Printf("Total cost: $%.4f\n", result.Result.Data.Cost.TotalCost)
	}
}