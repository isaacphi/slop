package llm

import "github.com/isaacphi/slop/internal/events"

// TextEvent represents a chunk of text from the LLM
type TextEvent struct {
	Content string
}

func (e TextEvent) Type() events.EventType {
	return events.EventTypeText
}

// ToolCallStartEvent represents a tool call starting in a stream
type ToolCallStartEvent struct {
	FunctionName string
}

func (e ToolCallStartEvent) Type() events.EventType {
	return events.EventTypeToolCallStart
}

// ToolNewArgumentEvent represents a new argument being streamed
type ToolNewArgumentEvent struct {
	ToolCallID   string
	Name         string
	ArgumentName string
}

func (e ToolNewArgumentEvent) Type() events.EventType {
	return events.EventTypeToolNewArgument
}

// ToolArgumentChunkEvent represents a chunk of a tool call
type ToolArgumentChunkEvent struct {
	ToolCallID   string
	Name         string
	ArgumentName string
	Chunk        string
}

func (e ToolArgumentChunkEvent) Type() events.EventType {
	return events.EventTypeToolArgumentChunk
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
