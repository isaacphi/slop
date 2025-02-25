package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/isaacphi/slop/internal/domain"
	"github.com/isaacphi/slop/internal/llm"
)

// PendingFunctionCallError is returned when a function call needs user approval
type PendingFunctionCallError struct {
	Message   *domain.Message
	ToolCalls []llm.ToolCall
}

func (e *PendingFunctionCallError) Error() string {
	return "Tool calls require approval"
}

// ApproveAndExecuteTools handles tool approval and execution, then continues the conversation
func (a *Agent) ApproveAndExecuteTools(ctx context.Context, messageID uuid.UUID, opts SendMessageOptions) (*domain.Message, error) {
	msg, err := a.repository.GetMessage(ctx, messageID)
	if err != nil {
		return nil, fmt.Errorf("failed to get message with pending tool call: %w", err)
	}

	if msg.Role != domain.RoleAssistant {
		return nil, fmt.Errorf("message must have role Assistant")
	}

	if msg.ToolCalls == "" {
		return nil, fmt.Errorf("no tool calls found in message")
	}

	// Parse tool calls
	var toolCalls []llm.ToolCall
	if err := json.Unmarshal([]byte(msg.ToolCalls), &toolCalls); err != nil {
		return nil, fmt.Errorf("failed to parse tool calls: %w", err)
	}

	if len(toolCalls) == 0 {
		return nil, fmt.Errorf("no tool calls found in message")
	}

	// Execute the tools
	results, err := a.ExecuteTools(ctx, toolCalls)
	if err != nil {
		return nil, fmt.Errorf("failed to execute tools: %w", err)
	}

	// Send results back to continue conversation
	return a.SendMessage(ctx, SendMessageOptions{
		ThreadID:      msg.ThreadID,
		ParentID:      &messageID,
		Content:       results,
		Role:          domain.RoleTool,
		StreamHandler: opts.StreamHandler,
	})
}

// RejectTools handles a denied function call
func (a *Agent) RejectTools(ctx context.Context, messageID uuid.UUID, reason string) (*domain.Message, error) {
	msg, err := a.repository.GetMessage(ctx, messageID)
	if err != nil {
		return nil, fmt.Errorf("failed to get message with pending tool call: %w", err)
	}

	if msg.Role != domain.RoleAssistant {
		return nil, fmt.Errorf("message must have role Assistant")
	}

	if msg.ToolCalls == "" {
		return nil, fmt.Errorf("no tool calls found in message")
	}

	// Parse tool calls
	var toolCalls []llm.ToolCall
	if err := json.Unmarshal([]byte(msg.ToolCalls), &toolCalls); err != nil {
		return nil, fmt.Errorf("failed to parse tool calls: %w", err)
	}

	if len(toolCalls) == 0 {
		return nil, fmt.Errorf("no tool calls found in message")
	}

	content := fmt.Sprintf("Function call denied: %s\nPlease suggest an alternative approach.", reason)
	return a.SendMessage(ctx, SendMessageOptions{
		ThreadID: msg.ThreadID,
		ParentID: &messageID,
		Content:  content,
	})
}
