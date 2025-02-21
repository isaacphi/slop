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
	"github.com/isaacphi/slop/internal/repository"
)

// "Agent" manages the interaction between the repository, llm, and function calls
type Agent struct {
	repository  repository.MessageRepository
	mcpClient   *mcp.Client
	modelConfig config.ModelPreset
	tools       map[string]mcp.Tool
	cfg         config.Agent
}

type SendMessageOptions struct {
	ThreadID      uuid.UUID
	ParentID      *uuid.UUID
	Content       string
	StreamHandler llm.StreamHandler
}

// New creates a new Agent with the given dependencies
func New(repo repository.MessageRepository, mcpClient *mcp.Client, modelCfg config.ModelPreset, cfg config.Agent) *Agent {
	return &Agent{
		repository:  repo,
		mcpClient:   mcpClient,
		modelConfig: modelCfg,
		tools:       mcpClient.GetTools(),
		cfg:         cfg,
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
	result, err := a.mcpClient.CallTool(ctx, toolCall.Name, args)
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

// SendMessage sends a message through the Agent, handling any function calls
func (a *Agent) SendMessage(ctx context.Context, opts SendMessageOptions) (*domain.Message, error) {
	// Verify thread exists
	thread, err := a.repository.GetThreadByID(ctx, opts.ThreadID)
	if err != nil {
		return nil, fmt.Errorf("failed to get thread: %w", err)
	}

	// If no parent specified, get the most recent message in thread
	if opts.ParentID == nil {
		messages, err := a.repository.GetMessages(ctx, thread.ID, nil, false)
		if err != nil {
			return nil, fmt.Errorf("failed to get messages: %w", err)
		}
		if len(messages) > 0 {
			lastMsg := messages[len(messages)-1]
			opts.ParentID = &lastMsg.ID
		}
	}

	// Get conversation history for context
	messages, err := a.repository.GetMessages(ctx, thread.ID, opts.ParentID, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get conversation history: %w", err)
	}

	// Create user message
	userMsg := &domain.Message{
		ThreadID: opts.ThreadID,
		ParentID: opts.ParentID,
		Role:     domain.RoleHuman,
		Content:  opts.Content,
	}

	if err := a.repository.AddMessageToThread(ctx, opts.ThreadID, userMsg); err != nil {
		return nil, err
	}

	// Get AI response
	aiResponse, err := llm.GenerateContent(
		ctx,
		a.modelConfig,
		opts.Content,
		messages,
		a.tools,
		opts.StreamHandler,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate AI response: %w", err)
	}

	if opts.StreamHandler != nil {
		_ = opts.StreamHandler.HandleMessageDone()
		opts.StreamHandler.Reset()
	}

	toolCallsString, err := json.Marshal(aiResponse.ToolCalls)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse ToolCalls: %w", err)
	}

	// Create AI message as a reply to the user message
	aiMsg := &domain.Message{
		ThreadID:  opts.ThreadID,
		ParentID:  &userMsg.ID,
		Role:      domain.RoleAssistant,
		Content:   aiResponse.TextResponse,
		ToolCalls: string(toolCallsString),
		ModelName: a.modelConfig.Name,
		Provider:  a.modelConfig.Provider,
	}

	if err := a.repository.AddMessageToThread(ctx, opts.ThreadID, aiMsg); err != nil {
		return nil, err
	}

	// Check for function calls in response
	if len(aiResponse.ToolCalls) == 0 {
		return aiMsg, nil
	}

	// Create channels for collecting results
	type toolResult struct {
		call   llm.ToolCall
		result string
		err    error
	}
	resultChan := make(chan toolResult, len(aiResponse.ToolCalls))

	// Launch concurrent execution of all tool calls
	for _, call := range aiResponse.ToolCalls {
		go func(tc llm.ToolCall) {
			if a.cfg.AutoApproveFunctions {
				result, err := a.executeFunction(ctx, tc, a.tools)
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

	for i := 0; i < len(aiResponse.ToolCalls); i++ {
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
		if i < len(aiResponse.ToolCalls)-1 {
			combinedResults.WriteString("\n")
		}
	}

	// If auto-approve is disabled, return for manual approval with first tool call
	if !a.cfg.AutoApproveFunctions {
		return aiMsg, &PendingFunctionCallError{
			Message:  aiMsg,
			ToolCall: aiResponse.ToolCalls[0],
		}
	}

	// Send combined results as followup message
	followupOpts := SendMessageOptions{
		ThreadID:      opts.ThreadID,
		ParentID:      &aiMsg.ID,
		Content:       combinedResults.String(),
		StreamHandler: opts.StreamHandler,
	}

	return a.SendMessage(ctx, followupOpts)
}

// DenyFunctionCall handles a denied function call
func (a *Agent) DenyFunctionCall(ctx context.Context, threadID uuid.UUID, messageID uuid.UUID, reason string) (*domain.Message, error) {
	content := fmt.Sprintf("Function call denied: %s\nPlease suggest an alternative approach.", reason)
	return a.SendMessage(ctx, SendMessageOptions{
		ThreadID: threadID,
		ParentID: &messageID,
		Content:  content,
	})
}
