// Package pkg provides a Go SDK for interacting with the Claude Code CLI.
//
// The SDK offers two main ways to interact with Claude:
//
// 1. Simple one-shot queries using the Query function
// 2. Full bidirectional communication using the Client
//
// Example usage for a simple query:
//
//	result, err := pkg.Query(ctx, "What is the capital of France?", nil)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println(result.Stdout)
//
// Example usage for interactive sessions:
//
//	client, err := pkg.NewClient(ctx, nil)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Close()
//
//	err = client.SendMessage(ctx, "Tell me about Go concurrency")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	for msg := range client.StreamMessages(ctx) {
//	    // Process messages
//	}
package pkg