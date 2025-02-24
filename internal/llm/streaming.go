package llm

import "github.com/isaacphi/slop/internal/events"

// TextEvent represents a chunk of text from the LLM
type TextEvent struct {
	Content string
}

func (e TextEvent) Type() events.EventType {
	return events.EventTypeText
}

// ToolCallEvent represents a chunk of a tool call
type ToolCallEvent struct {
	ToolCallID   string
	Name         string
	ArgumentName string
	Chunk        string
}

func (e ToolCallEvent) Type() events.EventType {
	return events.EventTypeToolCall
}

// LLMStream represents an ongoing LLM response stream
type LLMStream struct {
	Events <-chan events.Event
	Done   <-chan struct{}
}

// MessageCompleteEvent is sent when the LLM response is complete with all metadata
type MessageCompleteEvent struct {
	Content   string
	ToolCalls []ToolCall
}

func (e MessageCompleteEvent) Type() events.EventType {
	return events.EventTypeMessageComplete
}
