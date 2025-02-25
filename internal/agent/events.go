package agent

import (
	"github.com/isaacphi/slop/internal/domain"
	"github.com/isaacphi/slop/internal/events"
	"github.com/isaacphi/slop/internal/llm"
)

// ToolApprovalRequestEvent represents a tool call waiting for approval
type ToolApprovalRequestEvent struct {
	Message   *domain.Message
	ToolCalls []llm.ToolCall
}

func (e ToolApprovalRequestEvent) Type() events.EventType {
	return events.EventTypeToolApproval
}

// ToolResultEvent represents the result of a tool execution
type ToolResultEvent struct {
	ToolCallID string
	Name       string
	Result     string
	Error      error
}

func (e ToolResultEvent) Type() events.EventType {
	return events.EventTypeToolResult
}

// NewMessageEvent represents a completed message
type NewMessageEvent struct {
	Message *domain.Message
}

func (e NewMessageEvent) Type() events.EventType {
	return events.EventTypeNewMessage
}

// AgentStream represents an ongoing conversation stream
type AgentStream struct {
	Events <-chan events.Event
	Done   <-chan struct{}
}
