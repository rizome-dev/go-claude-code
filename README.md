# Claude Code Go SDK

A production-ready Go SDK for interacting with the Claude Code CLI, providing both simple one-shot queries and full bidirectional communication for interactive sessions.

## Features

- **Simple Query Interface**: Fire-and-forget queries with a single function call
- **Interactive Client**: Full bidirectional communication with streaming responses
- **Type-Safe**: Comprehensive type definitions for all messages and options
- **Concurrent**: Built with Go's concurrency primitives for efficient message handling
- **Error Handling**: Detailed error types for different failure scenarios
- **MCP Server Support**: Configure and use Model Context Protocol servers
- **Context Support**: Full context.Context integration for cancellation and timeouts

## Installation

```bash
go get github.com/rizome-dev/go-claude-code
```

### Prerequisites

You need to have the Claude Code CLI installed:

```bash
npm install -g @anthropic-ai/claude-code
```

Or download from: https://github.com/anthropics/claude-code/releases

## Quick Start

### Simple Query

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    "github.com/rizome-dev/go-claude-code/pkg"
)

func main() {
    ctx := context.Background()
    
    response, err := pkg.SimpleQuery(ctx, "What is the capital of France?")
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Println(response)
}
```

### Interactive Session

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    "github.com/rizome-dev/go-claude-code/pkg"
)

func main() {
    ctx := context.Background()
    
    client, err := pkg.NewClient(ctx, nil)
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()
    
    // Send a message
    err = client.SendMessage(ctx, "Explain Go concurrency")
    if err != nil {
        log.Fatal(err)
    }
    
    // Stream responses
    for msg := range client.StreamMessages(ctx) {
        switch m := msg.(type) {
        case *pkg.AssistantMessage:
            for _, block := range m.Content {
                if text, ok := block.(pkg.TextBlock); ok {
                    fmt.Print(text.Text)
                }
            }
        case pkg.ResultMessage:
            fmt.Printf("\nCost: $%.4f\n", m.Data.Cost.TotalCost)
            return
        }
    }
}
```

## API Reference

### Query Functions

#### `SimpleQuery(ctx, prompt) (string, error)`
Sends a simple query and returns the text response.

#### `Query(ctx, prompt, options) (*QueryResult, error)`
Sends a query with options and returns detailed results including messages and metadata.

#### `QueryWithOptions(ctx, prompt, optionsFn) (*QueryResult, error)`
Sends a query with a configuration function for setting options.

### Client API

#### `NewClient(ctx, options) (*Client, error)`
Creates a new interactive client with the specified options.

#### `Client.SendMessage(ctx, prompt) error`
Sends a user message to Claude.

#### `Client.SendInterrupt(ctx) error`
Sends an interrupt signal to stop Claude's current response.

#### `Client.StreamMessages(ctx) <-chan Message`
Returns a channel that streams all messages from Claude.

#### `Client.WaitForResult(ctx) (*ResultMessage, error)`
Blocks until a result message is received.

#### `Client.Close() error`
Closes the client and cleans up resources.

### Types

#### Message Types
- `UserMessage`: User input messages
- `AssistantMessage`: Claude's responses with content blocks
- `SystemMessage`: System-level messages (usage, thinking, errors)
- `ResultMessage`: Final result with usage and cost information

#### Content Blocks
- `TextBlock`: Plain text content
- `ToolUseBlock`: Tool invocation details
- `ToolResultBlock`: Tool execution results

#### Configuration Options
```go
type ClaudeCodeOptions struct {
    ApiKeyName          string
    BaseURL             string
    Model               string
    MaxTokens           int
    MaxBackgroundTokens int
    MaxCostUSD          float64
    Temperature         float64
    CustomInstructions  string
    SystemPrompt        string
    Mode                PermissionMode
    AssistantID         string
    OnlyTools           []string
    McpServers          map[string]MCPServerConfig
    MaxFileUploadsBytes int
    MaxImagePixels      int
    Cwd                 string
    SessionID           string
}
```

## Advanced Usage

### Configuring MCP Servers

```go
options := &pkg.ClaudeCodeOptions{
    McpServers: map[string]pkg.MCPServerConfig{
        "filesystem": {
            Type:    pkg.MCPServerTypeStdio,
            Command: "mcp-server-filesystem",
            Args:    []string{"/workspace"},
        },
        "api": {
            Type:   pkg.MCPServerTypeHTTP,
            URL:    "https://api.example.com/mcp",
            APIKey: "your-api-key",
        },
    },
}
```

### Error Handling

The SDK provides specific error types for different scenarios:

```go
result, err := pkg.Query(ctx, prompt, options)
if err != nil {
    var cliNotFound *errors.CLINotFoundError
    if errors.As(err, &cliNotFound) {
        // Claude Code CLI is not installed
        fmt.Println("Please install Claude Code CLI")
        return
    }
    
    var procErr *errors.ProcessError
    if errors.As(err, &procErr) {
        // CLI process failed
        fmt.Printf("Process failed with code %d: %s\n", 
            procErr.ExitCode, procErr.Stderr)
        return
    }
    
    // Handle other errors
    log.Fatal(err)
}
```

### Context and Timeouts

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

result, err := pkg.Query(ctx, "Complex task...", options)
if err == context.DeadlineExceeded {
    fmt.Println("Query timed out")
}
```

