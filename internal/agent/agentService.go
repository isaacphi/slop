package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/isaacphi/slop/internal/config"
	"github.com/isaacphi/slop/internal/domain"
	"github.com/isaacphi/slop/internal/llm"
	"github.com/isaacphi/slop/internal/mcp"
	"github.com/isaacphi/slop/internal/message"
)

// "Agent" manages the interaction between the message service and function calls
type Agent struct {
	messageService *message.MessageService
	mcp            *mcp.Client
	cfg            config.Agent
}

// New creates a new "Agent" with the given message service and configuration
func New(messageService *message.MessageService, mcpClient *mcp.Client, cfg config.Agent) *Agent {
	return &Agent{
		messageService: messageService,
		mcp:            mcpClient,
		cfg:            cfg,
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
func validateArguments(args json.RawMessage, tool mcp.Tool) error {
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
func (a *Agent) executeFunction(ctx context.Context, toolCall llm.ToolCall, tools map[string]mcp.Tool) (string, error) {
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

// SendMessage sends a message through the "Agent", handling any function calls
func (a *Agent) SendMessage(ctx context.Context, opts message.SendMessageOptions) (*domain.Message, error) {
	opts.Tools = a.mcp.GetTools()

	// Start with normal message flow
	responseMsg, err := a.messageService.SendMessage(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("message service error: %w", err)
	}

	_ = opts.StreamHandler.HandleMessageDone()

	// Reset stream handler
	if opts.StreamHandler != nil {
		opts.StreamHandler.Reset()
	}

	var toolCalls []llm.ToolCall
	err = json.Unmarshal([]byte(responseMsg.ToolCalls), &toolCalls)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling tool calls: %w", err)
	}

	// Check for function calls in response
	if len(toolCalls) == 0 {
		return responseMsg, nil
	}

	// Create channels for collecting results
	type toolResult struct {
		call   llm.ToolCall
		result string
		err    error
	}
	resultChan := make(chan toolResult, len(toolCalls))

	// Launch concurrent execution of all tool calls
	for _, call := range toolCalls {
		go func(tc llm.ToolCall) {
			if a.cfg.AutoApproveFunctions {
				result, err := a.executeFunction(ctx, tc, opts.Tools)
				resultChan <- toolResult{
					call:   tc,
					result: result,
					err:    err,
				}
			}
		}(call)
	}

	// Collect all results
	var combinedResults strings.Builder
	combinedResults.WriteString("Tool call results:\n\n")

	for i := 0; i < len(toolCalls); i++ {
		res := <-resultChan

		// Format the tool call header
		fmt.Fprintf(&combinedResults, "Name: %s\n", res.call.Name)
		fmt.Fprintf(&combinedResults, "ID: %s\n", res.call.ID)
		fmt.Fprintf(&combinedResults, "Arguments: %s\n", string(res.call.Arguments))
		fmt.Fprint(&combinedResults, "Result:\n")

		// Add result or error
		if res.err != nil {
			fmt.Fprintf(&combinedResults, "Error: %v\n", res.err)
		} else {
			fmt.Fprintf(&combinedResults, "%s\n", res.result)
		}

		// Add separator between results unless it's the last one
		if i < len(toolCalls)-1 {
			combinedResults.WriteString("\n")
		}
	}

	// If auto-approve is disabled, return for manual approval with first tool call
	if !a.cfg.AutoApproveFunctions {
		return responseMsg, &PendingFunctionCallError{
			Message:  responseMsg,
			ToolCall: toolCalls[0],
		}
	}

	// fmt.Printf("\n%s\n", combinedResults.String()[:150]+"...")

	// Send combined results as followup message
	followupOpts := message.SendMessageOptions{
		ThreadID:      opts.ThreadID,
		ParentID:      &responseMsg.ID,
		Content:       combinedResults.String(),
		StreamHandler: opts.StreamHandler,
	}

	return a.SendMessage(ctx, followupOpts)
}

// DenyFunctionCall handles a denied function call
func (a *Agent) DenyFunctionCall(ctx context.Context, threadID uuid.UUID, messageID uuid.UUID, reason string) (*domain.Message, error) {
	content := fmt.Sprintf("Function call denied: %s\nPlease suggest an alternative approach.", reason)
	return a.messageService.SendMessage(ctx, message.SendMessageOptions{
		ThreadID: threadID,
		ParentID: &messageID,
		Content:  content,
	})
}
