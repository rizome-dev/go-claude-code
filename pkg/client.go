package pkg

import (
	"context"
	"fmt"
	"sync"
)

type Client struct {
	transport   *transport
	options     *ClaudeCodeOptions
	messages    []Message
	mu          sync.Mutex
	closed      bool
	connected   bool
}

// NewClient creates a new client instance without connecting to the CLI.
// Call Connect() to establish the connection.
func NewClient(options *ClaudeCodeOptions) *Client {
	if options == nil {
		options = &ClaudeCodeOptions{}
	}

	return &Client{
		options:   options,
		messages:  make([]Message, 0),
		connected: false,
	}
}

// Connect establishes a connection to the Claude CLI.
// If prompt is provided, it will be sent as the initial message.
func (c *Client) Connect(ctx context.Context, prompt string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return fmt.Errorf("client is already connected")
	}

	if c.closed {
		return fmt.Errorf("client is closed")
	}

	transport, err := newTransport(ctx, c.options, true)
	if err != nil {
		return err
	}

	c.transport = transport
	c.connected = true

	// If a prompt is provided, send it as the initial message
	if prompt != "" {
		msg := UserMessage{
			Role:    MessageRoleUser,
			Content: prompt,
		}
		return c.transport.sendMessage(ctx, msg, "", c.options.SessionID)
	}

	return nil
}

func (c *Client) SendMessage(ctx context.Context, prompt string) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return fmt.Errorf("client is closed")
	}
	if !c.connected {
		c.mu.Unlock()
		return fmt.Errorf("client is not connected, call Connect() first")
	}
	c.mu.Unlock()

	msg := UserMessage{
		Role:    MessageRoleUser,
		Content: prompt,
	}

	return c.transport.sendMessage(ctx, msg, "", c.options.SessionID)
}

func (c *Client) SendInterrupt(ctx context.Context) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return fmt.Errorf("client is closed")
	}
	if !c.connected {
		c.mu.Unlock()
		return fmt.Errorf("client is not connected, call Connect() first")
	}
	c.mu.Unlock()

	return c.transport.sendInterrupt(ctx)
}

func (c *Client) Messages() <-chan Message {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if !c.connected || c.transport == nil {
		// Return a closed channel if not connected
		ch := make(chan Message)
		close(ch)
		return ch
	}
	return c.transport.messages
}

func (c *Client) Errors() <-chan error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if !c.connected || c.transport == nil {
		// Return a closed channel if not connected
		ch := make(chan error)
		close(ch)
		return ch
	}
	return c.transport.errors
}

func (c *Client) GetMessages() []Message {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	result := make([]Message, len(c.messages))
	copy(result, c.messages)
	return result
}

func (c *Client) StreamMessages(ctx context.Context) <-chan Message {
	out := make(chan Message)
	
	c.mu.Lock()
	if !c.connected || c.transport == nil {
		c.mu.Unlock()
		close(out)
		return out
	}
	msgChan := c.transport.messages
	c.mu.Unlock()
	
	go func() {
		defer close(out)
		
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-msgChan:
				if !ok {
					return
				}
				
				c.mu.Lock()
				c.messages = append(c.messages, msg)
				c.mu.Unlock()
				
				select {
				case out <- msg:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	
	return out
}

func (c *Client) WaitForResult(ctx context.Context) (*ResultMessage, error) {
	c.mu.Lock()
	if !c.connected || c.transport == nil {
		c.mu.Unlock()
		return nil, fmt.Errorf("client is not connected, call Connect() first")
	}
	msgChan := c.transport.messages
	errChan := c.transport.errors
	c.mu.Unlock()
	
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case err := <-errChan:
			return nil, err
		case msg, ok := <-msgChan:
			if !ok {
				return nil, fmt.Errorf("message channel closed")
			}
			
			c.mu.Lock()
			c.messages = append(c.messages, msg)
			c.mu.Unlock()
			
			if result, ok := msg.(ResultMessage); ok {
				return &result, nil
			}
		}
	}
}

// ReceiveResponse receives messages until a ResultMessage is encountered.
// Returns a channel that yields all messages including the ResultMessage.
// The channel is closed after the ResultMessage is sent.
func (c *Client) ReceiveResponse(ctx context.Context) <-chan Message {
	out := make(chan Message)
	
	c.mu.Lock()
	if !c.connected || c.transport == nil {
		c.mu.Unlock()
		close(out)
		return out
	}
	msgChan := c.transport.messages
	errChan := c.transport.errors
	c.mu.Unlock()
	
	go func() {
		defer close(out)
		
		for {
			select {
			case <-ctx.Done():
				return
			case <-errChan:
				// Could consider sending error as a special message type
				// For now, just close the channel on error
				return
			case msg, ok := <-msgChan:
				if !ok {
					return
				}
				
				c.mu.Lock()
				c.messages = append(c.messages, msg)
				c.mu.Unlock()
				
				select {
				case out <- msg:
				case <-ctx.Done():
					return
				}
				
				// Check if this is a ResultMessage
				if _, isResult := msg.(ResultMessage); isResult {
					return
				}
			}
		}
	}()
	
	return out
}

// ReceiveMessages receives all messages from the transport.
// This is equivalent to Python's receive_messages() method.
func (c *Client) ReceiveMessages(ctx context.Context) <-chan Message {
	return c.StreamMessages(ctx)
}

func (c *Client) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	transport := c.transport
	c.connected = false
	c.mu.Unlock()

	if transport != nil {
		return transport.close()
	}
	return nil
}

type MessageIterator struct {
	client *Client
	ctx    context.Context
}

func (c *Client) IterateMessages(ctx context.Context) *MessageIterator {
	return &MessageIterator{
		client: c,
		ctx:    ctx,
	}
}

func (it *MessageIterator) Next() (Message, error) {
	select {
	case <-it.ctx.Done():
		return nil, it.ctx.Err()
	case err := <-it.client.transport.errors:
		return nil, err
	case msg, ok := <-it.client.transport.messages:
		if !ok {
			return nil, fmt.Errorf("message channel closed")
		}
		
		it.client.mu.Lock()
		it.client.messages = append(it.client.messages, msg)
		it.client.mu.Unlock()
		
		return msg, nil
	}
}