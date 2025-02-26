package llm

import "github.com/isaacphi/slop/internal/events"

// TextEvent represents a chunk of text from the LLM
type TextEvent struct {
	Content string
}

func (e TextEvent) Type() events.EventType {
	return events.EventTypeText
}

type JsonUpdateEvent struct {
	Key        string // The json style path of the key that is receiving an update
	ValueChunk string // Incremental update to that key
}

func (e JsonUpdateEvent) Type() events.EventType {
	return events.EventTypeJsonUpdate
}

// ToolCallStartEvent represents a tool call starting in a stream
type ToolCallStartEvent struct {
	FunctionName string
}

func (e ToolCallStartEvent) Type() events.EventType {
	return events.EventTypeToolCallStart
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
