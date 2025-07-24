package pkg

import (
	"fmt"
)

type ClaudeSDKError struct {
	Message string
	Cause   error
}

func (e *ClaudeSDKError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *ClaudeSDKError) Unwrap() error {
	return e.Cause
}

type CLIConnectionError struct {
	ClaudeSDKError
}

func NewCLIConnectionError(message string, cause error) *CLIConnectionError {
	return &CLIConnectionError{
		ClaudeSDKError: ClaudeSDKError{
			Message: message,
			Cause:   cause,
		},
	}
}

type CLINotFoundError struct {
	ClaudeSDKError
	SearchPaths []string
}

func NewCLINotFoundError(searchPaths []string) *CLINotFoundError {
	message := `Claude Code CLI not found. Please install it first:

Option 1: Install via npm (requires Node.js 18+):
  npm install -g @anthropic-ai/claude-code

Option 2: Download from GitHub:
  https://github.com/anthropics/claude-code/releases

After installation, make sure 'claude-code' is in your PATH.`

	return &CLINotFoundError{
		ClaudeSDKError: ClaudeSDKError{
			Message: message,
		},
		SearchPaths: searchPaths,
	}
}

type ProcessError struct {
	ClaudeSDKError
	ExitCode int
	Stdout   string
	Stderr   string
}

func NewProcessError(exitCode int, stdout, stderr string) *ProcessError {
	message := fmt.Sprintf("Claude Code CLI exited with code %d", exitCode)
	if stderr != "" {
		message = fmt.Sprintf("%s\nstderr: %s", message, stderr)
	}
	
	return &ProcessError{
		ClaudeSDKError: ClaudeSDKError{
			Message: message,
		},
		ExitCode: exitCode,
		Stdout:   stdout,
		Stderr:   stderr,
	}
}

type CLIJSONDecodeError struct {
	ClaudeSDKError
	RawData string
}

func NewCLIJSONDecodeError(rawData string, cause error) *CLIJSONDecodeError {
	return &CLIJSONDecodeError{
		ClaudeSDKError: ClaudeSDKError{
			Message: fmt.Sprintf("Failed to decode JSON from CLI output: %s", rawData),
			Cause:   cause,
		},
		RawData: rawData,
	}
}

type MessageParseError struct {
	ClaudeSDKError
	MessageType string
	RawMessage  interface{}
}

func NewMessageParseError(messageType string, rawMessage interface{}, cause error) *MessageParseError {
	return &MessageParseError{
		ClaudeSDKError: ClaudeSDKError{
			Message: fmt.Sprintf("Failed to parse message of type '%s'", messageType),
			Cause:   cause,
		},
		MessageType: messageType,
		RawMessage:  rawMessage,
	}
}