package events

// EventType defines the type of streaming event
type EventType int

const (
	EventTypeText EventType = iota
	EventTypeToolCallStart
	EventTypeJsonUpdate
	EventTypeToolNewArgument
	EventTypeToolArgumentChunk
	EventTypeToolApproval
	EventTypeToolResult
	EventTypeNewMessage
	EventTypeError
	EventTypeMessageComplete
)

// Event is the interface for all streaming events
type Event interface {
	Type() EventType
}

// ErrorEvent represents an error during processing
type ErrorEvent struct {
	Error error
}

func (e ErrorEvent) Type() EventType {
	return EventTypeError
}
