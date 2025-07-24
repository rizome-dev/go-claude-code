package pkg

import (
	"encoding/json"
	"fmt"
)

type messageParser struct{}

func newMessageParser() *messageParser {
	return &messageParser{}
}

func (p *messageParser) parseStreamMessage(data []byte) (*StreamMessage, error) {
	var msg StreamMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, NewCLIJSONDecodeError(string(data), err)
	}
	return &msg, nil
}

func (p *messageParser) parseControlResponse(data []byte) (*ControlResponse, error) {
	var resp ControlResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, NewCLIJSONDecodeError(string(data), err)
	}
	return &resp, nil
}

func (p *messageParser) parseMessage(msgType string, data json.RawMessage) (Message, error) {
	// Skip empty or null messages gracefully
	if len(data) == 0 || string(data) == "null" || string(data) == "{}" {
		return nil, nil
	}

	switch msgType {
	case "user":
		var msg UserMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, NewMessageParseError(msgType, string(data), err)
		}
		return msg, nil

	case "assistant":
		var msg AssistantMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, NewMessageParseError(msgType, string(data), err)
		}
		return &msg, nil

	case "system":
		var base struct {
			Subtype string `json:"subtype"`
		}
		if err := json.Unmarshal(data, &base); err != nil {
			return nil, NewMessageParseError(msgType, string(data), err)
		}

		if base.Subtype == "result" {
			var msg ResultMessage
			if err := json.Unmarshal(data, &msg); err != nil {
				return nil, NewMessageParseError(msgType, string(data), err)
			}
			return msg, nil
		}

		var msg SystemMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, NewMessageParseError(msgType, string(data), err)
		}
		return msg, nil

	default:
		return nil, NewMessageParseError(msgType, string(data), 
			fmt.Errorf("unknown message type: %s", msgType))
	}
}

func (p *messageParser) isControlResponse(data []byte) bool {
	var check struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &check); err != nil {
		return false
	}
	return check.Type == "control_response"
}