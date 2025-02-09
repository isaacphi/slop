package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/isaacphi/slop/internal/config"
	"github.com/isaacphi/slop/internal/domain"
	"github.com/isaacphi/slop/internal/llm"
	"github.com/isaacphi/slop/internal/mcp"
	"github.com/isaacphi/slop/internal/message"
)

// "Agent" manages the interaction between the message service and function calls
type Agent struct {
	messageService *service.MessageService
	mcp            *mcp.Client
	config         *config.ConfigSchema
}

// New creates a new "Agent" with the given message service and configuration
func New(messageService *service.MessageService, mcpClient *mcp.Client, cfg *config.ConfigSchema) *Agent {
	return &Agent{
		messageService: messageService,
		mcp:            mcpClient,
		config:         cfg,
	}
}

// PendingFunctionCallError is returned when a function call needs user approval
type PendingFunctionCallError struct {
	Message  *domain.Message
	ToolCall llm.ToolCall
}

func (e *PendingFunctionCallError) Error() string {
	return fmt.Sprintf("pending function call approval for %s", e.ToolCall.Name)
}

// validateArguments checks if the provided arguments match the tool's schema
func validateArguments(args json.RawMessage, tool config.Tool) error {
	var parsedArgs map[string]interface{}
	if err := json.Unmarshal(args, &parsedArgs); err != nil {
		return fmt.Errorf("invalid argument format: %w", err)
	}

	// Check required parameters
	for _, required := range tool.Parameters.Required {
		if _, exists := parsedArgs[required]; !exists {
			return fmt.Errorf("missing required parameter: %s", required)
		}
	}

	// Validate each provided argument
	for name, value := range parsedArgs {
		prop, exists := tool.Parameters.Properties[name]
		if !exists {
			return fmt.Errorf("unknown parameter: %s", name)
		}

		// Type validation
		switch prop.Type {
		case "string":
			if _, ok := value.(string); !ok {
				return fmt.Errorf("parameter %s must be a string", name)
			}
		case "number":
			if _, ok := value.(float64); !ok {
				return fmt.Errorf("parameter %s must be a number", name)
			}
		case "boolean":
			if _, ok := value.(bool); !ok {
				return fmt.Errorf("parameter %s must be a boolean", name)
			}
		case "array":
			if _, ok := value.([]interface{}); !ok {
				return fmt.Errorf("parameter %s must be an array", name)
			}
		}

		// Enum validation
		if len(prop.Enum) > 0 {
			if strVal, ok := value.(string); ok {
				valid := false
				for _, enum := range prop.Enum {
					if strVal == enum {
						valid = true
						break
					}
				}
				if !valid {
					return fmt.Errorf("parameter %s must be one of: %v", name, prop.Enum)
				}
			}
		}
	}

	return nil
}

// executeFunction executes a function call and returns its result
func (a *Agent) executeFunction(ctx context.Context, toolCall llm.ToolCall, tools map[string]config.Tool) (string, error) {
	tool, exists := tools[toolCall.Name]
	if !exists {
		return "", fmt.Errorf("function %s not found", toolCall.Name)
	}

	if err := validateArguments(toolCall.Arguments, tool); err != nil {
		return "", fmt.Errorf("argument validation failed: %w", err)
	}

	// Parse arguments into interface{}
	var args interface{}
	if err := json.Unmarshal(toolCall.Arguments, &args); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	// Execute the function through MCP client
	result, err := a.mcp.CallTool(ctx, toolCall.Name, args)
	if err != nil {
		return "", fmt.Errorf("function execution failed: %w", err)
	}

	// Convert result to string
	resultBytes, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to format result: %w", err)
	}

	return string(resultBytes), nil
}

// formatFunctionResult formats a function result for sending back to the LLM
func formatFunctionResult(result string) string {
	return fmt.Sprintf("Function executed with result:\n\n%s\n", result)
}

// SendMessage sends a message through the "Agent", handling any function calls
func (a *Agent) SendMessage(ctx context.Context, opts service.SendMessageOptions) (*domain.Message, error) {
	opts.Tools = a.mcp.GetTools()

	// Start with normal message flow
	msg, err := a.messageService.SendMessage(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("message service error: %w", err)
	}

	var toolCalls []llm.ToolCall
	err = json.Unmarshal([]byte(msg.ToolCalls), &toolCalls)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling tool calls: %w", err)
	}

	// Check for function calls in response
	if len(toolCalls) == 0 {
		return msg, nil
	}

	toolCall := toolCalls[0]

	// Handle function call based on auto-approve setting
	if a.config.AutoApproveFunctions {
		result, err := a.executeFunction(ctx, toolCall, opts.Tools)
		if err != nil {
			return nil, fmt.Errorf("function execution error: %w", err)
		}

		// TODO: followups should stream and use tools

		// Feed result back to message
		followupOpts := service.SendMessageOptions{
			ThreadID: opts.ThreadID,
			ParentID: &msg.ID,
			Content:  formatFunctionResult(result),
		}
		return a.messageService.SendMessage(ctx, followupOpts)
	}

	// Return function call for manual approval
	return msg, &PendingFunctionCallError{
		Message:  msg,
		ToolCall: toolCall,
	}
}

// ApproveFunctionCall executes a previously pending function call
func (a *Agent) ApproveFunctionCall(ctx context.Context, threadID uuid.UUID, messageID uuid.UUID, tools map[string]config.Tool) (*domain.Message, error) {
	// TODO: handle multiple function calls or no function calls

	// Get the original message
	messages, err := a.messageService.GetThreadMessages(ctx, threadID, &messageID)
	if err != nil || len(messages) == 0 {
		return nil, fmt.Errorf("failed to get original message: %w", err)
	}

	var toolCalls []llm.ToolCall
	err = json.Unmarshal([]byte(messages[0].ToolCalls), &toolCalls)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling tool calls: %w", err)
	}

	result, err := a.executeFunction(ctx, toolCalls[0], tools)
	if err != nil {
		return nil, fmt.Errorf("function execution error: %w", err)
	}

	// Send result back to chat
	return a.messageService.SendMessage(ctx, service.SendMessageOptions{
		ThreadID: threadID,
		ParentID: &messageID,
		Content:  formatFunctionResult(result),
	})
}

// DenyFunctionCall handles a denied function call
func (a *Agent) DenyFunctionCall(ctx context.Context, threadID uuid.UUID, messageID uuid.UUID, reason string) (*domain.Message, error) {
	content := fmt.Sprintf("Function call denied: %s\nPlease suggest an alternative approach.", reason)
	return a.messageService.SendMessage(ctx, service.SendMessageOptions{
		ThreadID: threadID,
		ParentID: &messageID,
		Content:  content,
	})
}

// The following methods mirror the MessageService interface for convenience

func (a *Agent) NewThread(ctx context.Context) (*domain.Thread, error) {
	return a.messageService.NewThread(ctx)
}

func (a *Agent) GetActiveThread(ctx context.Context) (*domain.Thread, error) {
	return a.messageService.GetActiveThread(ctx)
}

func (a *Agent) ListThreads(ctx context.Context, limit int) ([]*domain.Thread, error) {
	return a.messageService.ListThreads(ctx, limit)
}

func (a *Agent) GetThreadMessages(ctx context.Context, threadID uuid.UUID, messageID *uuid.UUID) ([]domain.Message, error) {
	return a.messageService.GetThreadMessages(ctx, threadID, messageID)
}
