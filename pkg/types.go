package pkg

import (
	"encoding/json"
)

type PermissionMode string

const (
	PermissionModeDefault          PermissionMode = "default"
	PermissionModeAcceptEdits      PermissionMode = "acceptEdits"
	PermissionModeBypassPermissions PermissionMode = "bypassPermissions"
)

type MCPServerType string

const (
	MCPServerTypeStdio MCPServerType = "stdio"
	MCPServerTypeSSE   MCPServerType = "sse"
	MCPServerTypeHTTP  MCPServerType = "http"
)

type MCPServerConfig struct {
	Type     MCPServerType
	Command  string
	Args     []string
	Env      map[string]string
	URL      string
	APIKey   string
	Headers  map[string]string
}

func (c *MCPServerConfig) UnmarshalJSON(data []byte) error {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	serverType, ok := raw["type"].(string)
	if !ok {
		return json.Unmarshal(data, &struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
			Env     map[string]string `json:"env,omitempty"`
		}{
			Command: c.Command,
			Args:    c.Args,
			Env:     c.Env,
		})
	}

	c.Type = MCPServerType(serverType)
	switch c.Type {
	case MCPServerTypeStdio:
		c.Command, _ = raw["command"].(string)
		if args, ok := raw["args"].([]interface{}); ok {
			c.Args = make([]string, len(args))
			for i, arg := range args {
				c.Args[i], _ = arg.(string)
			}
		}
		if env, ok := raw["env"].(map[string]interface{}); ok {
			c.Env = make(map[string]string)
			for k, v := range env {
				c.Env[k], _ = v.(string)
			}
		}
	case MCPServerTypeSSE, MCPServerTypeHTTP:
		c.URL, _ = raw["url"].(string)
		c.APIKey, _ = raw["apiKey"].(string)
		if headers, ok := raw["headers"].(map[string]interface{}); ok {
			c.Headers = make(map[string]string)
			for k, v := range headers {
				c.Headers[k], _ = v.(string)
			}
		}
	}

	return nil
}

type ClaudeCodeOptions struct {
	// Python SDK compatible fields
	AllowedTools              []string                   `json:"allowedTools,omitempty"`
	MaxThinkingTokens         int                        `json:"maxThinkingTokens,omitempty"`
	SystemPrompt              string                     `json:"systemPrompt,omitempty"`
	AppendSystemPrompt        string                     `json:"appendSystemPrompt,omitempty"`
	MCPTools                  []string                   `json:"mcpTools,omitempty"`
	PermissionMode            PermissionMode             `json:"permissionMode,omitempty"`
	ContinueConversation      bool                       `json:"continueConversation,omitempty"`
	Resume                    string                     `json:"resume,omitempty"`
	MaxTurns                  int                        `json:"maxTurns,omitempty"`
	DisallowedTools           []string                   `json:"disallowedTools,omitempty"`
	Model                     string                     `json:"model,omitempty"`
	PermissionPromptToolName  string                     `json:"permissionPromptToolName,omitempty"`
	Cwd                       string                     `json:"cwd,omitempty"`
	
	// Additional Go SDK fields (kept for compatibility)
	ApiKeyName          string                     `json:"apiKeyName,omitempty"`
	BaseURL             string                     `json:"baseUrl,omitempty"`
	MaxTokens           int                        `json:"maxTokens,omitempty"`
	MaxBackgroundTokens int                        `json:"maxBackgroundTokens,omitempty"`
	MaxCostUSD          float64                    `json:"maxCostUsd,omitempty"`
	Temperature         float64                    `json:"temperature,omitempty"`
	CustomInstructions  string                     `json:"customInstructions,omitempty"`
	Mode                PermissionMode             `json:"mode,omitempty"` // Deprecated: use PermissionMode
	AssistantID         string                     `json:"assistantId,omitempty"`
	OnlyTools           []string                   `json:"onlyTools,omitempty"` // Deprecated: use AllowedTools
	McpServers          map[string]MCPServerConfig `json:"mcpServers,omitempty"`
	MaxFileUploadsBytes int                        `json:"maxFileUploadsBytes,omitempty"`
	MaxImagePixels      int                        `json:"maxImagePixels,omitempty"`
	SessionID           string                     `json:"sessionId,omitempty"`
}

type MessageRole string

const (
	MessageRoleUser      MessageRole = "user"
	MessageRoleAssistant MessageRole = "assistant"
	MessageRoleSystem    MessageRole = "system"
)

type Message interface {
	GetRole() MessageRole
	GetType() string
}

type UserMessage struct {
	Role    MessageRole `json:"role"`
	Content string      `json:"content"`
}

func (m UserMessage) GetRole() MessageRole { return m.Role }
func (m UserMessage) GetType() string      { return "user" }

type ContentBlock interface {
	GetType() string
}

type TextBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func (b TextBlock) GetType() string { return "text" }

type ToolUseBlock struct {
	Type  string                 `json:"type"`
	ID    string                 `json:"id"`
	Name  string                 `json:"name"`
	Input map[string]interface{} `json:"input"`
}

func (b ToolUseBlock) GetType() string { return "tool_use" }

type ToolResultBlock struct {
	Type       string                 `json:"type"`
	ToolUseID  string                 `json:"tool_use_id"`
	IsError    bool                   `json:"is_error,omitempty"`
	Content    interface{}            `json:"content"`
}

func (b ToolResultBlock) GetType() string { return "tool_result" }

type AssistantMessage struct {
	Role    MessageRole    `json:"role"`
	Content []ContentBlock `json:"content"`
}

func (m AssistantMessage) GetRole() MessageRole { return m.Role }
func (m AssistantMessage) GetType() string      { return "assistant" }

func (m *AssistantMessage) UnmarshalJSON(data []byte) error {
	type Alias AssistantMessage
	aux := &struct {
		Content []json.RawMessage `json:"content"`
		*Alias
	}{
		Alias: (*Alias)(m),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	m.Content = make([]ContentBlock, 0, len(aux.Content))
	for _, raw := range aux.Content {
		var typeCheck struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(raw, &typeCheck); err != nil {
			continue
		}

		var block ContentBlock
		switch typeCheck.Type {
		case "text":
			var tb TextBlock
			if err := json.Unmarshal(raw, &tb); err == nil {
				block = tb
			}
		case "tool_use":
			var tub ToolUseBlock
			if err := json.Unmarshal(raw, &tub); err == nil {
				block = tub
			}
		case "tool_result":
			var trb ToolResultBlock
			if err := json.Unmarshal(raw, &trb); err == nil {
				block = trb
			}
		}

		if block != nil {
			m.Content = append(m.Content, block)
		}
	}

	return nil
}

type SystemMessageSubtype string

const (
	SystemMessageSubtypeUsage         SystemMessageSubtype = "usage"
	SystemMessageSubtypeThinking      SystemMessageSubtype = "thinking"
	SystemMessageSubtypeLookup        SystemMessageSubtype = "lookup"
	SystemMessageSubtypeControl       SystemMessageSubtype = "control"
	SystemMessageSubtypeModelError    SystemMessageSubtype = "model_error"
	SystemMessageSubtypeMCPServerLog  SystemMessageSubtype = "mcp_server_log"
	SystemMessageSubtypeFile          SystemMessageSubtype = "file"
	SystemMessageSubtypeInterrupted   SystemMessageSubtype = "interrupted"
	SystemMessageSubtypeUserPromptSubmitHook SystemMessageSubtype = "user_prompt_submit_hook"
)

type SystemMessage struct {
	Role    MessageRole          `json:"role"`
	Subtype SystemMessageSubtype `json:"subtype"`
	Data    interface{}          `json:"data,omitempty"`
}

func (m SystemMessage) GetRole() MessageRole { return m.Role }
func (m SystemMessage) GetType() string      { return "system" }

type ResultUsage struct {
	InputTokens       int `json:"inputTokens"`
	OutputTokens      int `json:"outputTokens"`
	BackgroundTokens  int `json:"backgroundTokens"`
	CacheCreationTokens int `json:"cacheCreationTokens,omitempty"`
	CacheReadTokens   int `json:"cacheReadTokens,omitempty"`
}

type ResultCost struct {
	InputTokenCost       float64 `json:"inputTokenCost"`
	OutputTokenCost      float64 `json:"outputTokenCost"`
	BackgroundTokenCost  float64 `json:"backgroundTokenCost"`
	CacheCreationCost    float64 `json:"cacheCreationCost,omitempty"`
	CacheReadCost        float64 `json:"cacheReadCost,omitempty"`
	TotalCost            float64 `json:"totalCost"`
}

type ResultMessageData struct {
	Usage              ResultUsage `json:"usage"`
	Cost               ResultCost  `json:"cost"`
	SessionID          string      `json:"sessionId"`
	InterruptRequested bool        `json:"interruptRequested"`
}

type ResultMessage struct {
	Role MessageRole       `json:"role"`
	Data ResultMessageData `json:"data"`
}

func (m ResultMessage) GetRole() MessageRole { return MessageRoleSystem }
func (m ResultMessage) GetType() string      { return "result" }

type InputMessage struct {
	Type               string        `json:"type"`
	Message            Message       `json:"message"`
	ParentToolUseID    string        `json:"parent_tool_use_id,omitempty"`
	SessionID          string        `json:"session_id,omitempty"`
}

type ControlRequestType string

const (
	ControlRequestTypeInterrupt ControlRequestType = "interrupt"
)

type ControlRequest struct {
	Type      string             `json:"type"`
	RequestID string             `json:"request_id"`
	Request   struct {
		Subtype ControlRequestType `json:"subtype"`
	} `json:"request"`
}

type ControlResponse struct {
	Type      string `json:"type"`
	RequestID string `json:"request_id"`
	Response  struct {
		Success bool   `json:"success"`
		Error   string `json:"error,omitempty"`
	} `json:"response"`
}

type StreamMessage struct {
	Type    string          `json:"type"`
	Message json.RawMessage `json:"message"`
}

func (s *StreamMessage) Parse() (Message, error) {
	return parseMessage(s.Type, s.Message)
}

func parseMessage(msgType string, data json.RawMessage) (Message, error) {
	switch msgType {
	case "user":
		var msg UserMessage
		err := json.Unmarshal(data, &msg)
		return msg, err
	case "assistant":
		var msg AssistantMessage
		err := json.Unmarshal(data, &msg)
		return &msg, err
	case "system":
		var base struct {
			Subtype string `json:"subtype"`
		}
		if err := json.Unmarshal(data, &base); err != nil {
			return nil, err
		}

		if base.Subtype == "result" {
			var msg ResultMessage
			err := json.Unmarshal(data, &msg)
			return msg, err
		}

		var msg SystemMessage
		err := json.Unmarshal(data, &msg)
		return msg, err
	default:
		return nil, nil
	}
}