package pkg

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type QueryResult struct {
	Messages []Message
	Result   *ResultMessage
	Stdout   string
	Stderr   string
}

func Query(ctx context.Context, prompt string, options *ClaudeCodeOptions) (*QueryResult, error) {
	if options == nil {
		options = &ClaudeCodeOptions{}
	}

	// For query, we want non-streaming mode with prompt passed via --print flag
	transport, err := newTransportForQuery(ctx, options, prompt)
	if err != nil {
		return nil, err
	}
	defer transport.close()

	// Since we're using --print flag, we don't need to send message via stdin
	// Just close stdin immediately as Python does with close_stdin_after_prompt=True
	if err := transport.closeStdin(); err != nil {
		return nil, err
	}

	result := &QueryResult{
		Messages: make([]Message, 0),
	}

	messageChan := transport.messages
	errorChan := transport.errors
	
	waitDone := make(chan error, 1)
	go func() {
		waitDone <- transport.wait()
	}()

	timeout := time.NewTimer(30 * time.Minute)
	defer timeout.Stop()

Loop:
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeout.C:
			return nil, fmt.Errorf("query timeout after 30 minutes")
		case err := <-errorChan:
			if err != nil {
				return nil, err
			}
		case msg, ok := <-messageChan:
			if ok {
				result.Messages = append(result.Messages, msg)
				if res, isResult := msg.(ResultMessage); isResult {
					result.Result = &res
				}
			}
		case err := <-waitDone:
			if err != nil {
				return nil, err
			}
			break Loop
		}
	}

	stderr := transport.collectStderr(1 * time.Second)
	if stderr != "" {
		result.Stderr = stderr
	}

	var textParts []string
	for _, msg := range result.Messages {
		switch m := msg.(type) {
		case *AssistantMessage:
			for _, block := range m.Content {
				if text, ok := block.(TextBlock); ok {
					textParts = append(textParts, text.Text)
				}
			}
		}
	}
	
	if len(textParts) > 0 {
		result.Stdout = strings.Join(textParts, "\n")
	}

	return result, nil
}

func SimpleQuery(ctx context.Context, prompt string) (string, error) {
	result, err := Query(ctx, prompt, nil)
	if err != nil {
		return "", err
	}
	return result.Stdout, nil
}

func QueryWithOptions(ctx context.Context, prompt string, optionsFn func(*ClaudeCodeOptions)) (*QueryResult, error) {
	options := &ClaudeCodeOptions{}
	if optionsFn != nil {
		optionsFn(options)
	}
	return Query(ctx, prompt, options)
}