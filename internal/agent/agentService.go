package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
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
	repository repository.MessageRepository
	mcpClient  *mcp.Client
	preset     config.Preset
	tools      map[string]map[string]toolWithApproval // MCPServer -> Tool -> Tool Configuration
	toolsets   map[string]config.Toolset
	prompts    map[string]config.Prompt
}

type toolWithApproval struct {
	domain.Tool
	RequireApproval bool
}

type SendMessageOptions struct {
	ThreadID      uuid.UUID
	ParentID      *uuid.UUID
	Content       string
	StreamHandler llm.StreamHandler
	Role          domain.Role
}

// New creates a new Agent with the given dependencies
func New(
	repo repository.MessageRepository,
	mcpClient *mcp.Client,
	preset config.Preset,
	toolsets map[string]config.Toolset,
	prompts map[string]config.Prompt,
) (*Agent, error) {
	tools, err := filterAndModifyTools(mcpClient.GetTools(), preset.Toolsets, toolsets)

	if err != nil {
		return nil, fmt.Errorf("failed to process toolsets: %w", err)
	}

	return &Agent{
		repository: repo,
		mcpClient:  mcpClient,
		preset:     preset,
		tools:      tools,
		toolsets:   toolsets,
		prompts:    prompts,
	}, nil
}

func flattenTools(tools map[string]map[string]toolWithApproval) map[string]domain.Tool {
	flat := make(map[string]domain.Tool)
	for server, serverTools := range tools {
		for name, tool := range serverTools {
			flat[fmt.Sprintf("%s__%s", server, name)] = tool.Tool
		}
	}
	return flat
}

func filterAndModifyTools(allTools map[string]map[string]domain.Tool, modelToolsets []string, toolsets map[string]config.Toolset) (map[string]map[string]toolWithApproval, error) {
	result := make(map[string]map[string]toolWithApproval)

	for _, toolsetName := range modelToolsets {
		toolset := toolsets[toolsetName]

		for serverName, serverConfig := range toolset.Servers {
			serverTools, exists := allTools[serverName]
			if !exists {
				return nil, fmt.Errorf("server %q not found", serverName)
			}

			if _, exists := result[serverName]; !exists {
				result[serverName] = make(map[string]toolWithApproval)
			}

			// If AllowedTools is empty, include all server tools with server-level approval
			if len(serverConfig.AllowedTools) == 0 {
				for toolName, tool := range serverTools {
					result[serverName][toolName] = toolWithApproval{
						Tool:            tool,
						RequireApproval: serverConfig.RequireApproval,
					}
				}
				continue
			}

			// Process specific allowed tools
			for toolName, toolConfig := range serverConfig.AllowedTools {
				tool, exists := serverTools[toolName]
				if !exists {
					return nil, fmt.Errorf("tool %q not found in server %q", toolName, serverName)
				}

				if len(toolConfig.PresetParameters) > 0 {
					tool = modifyToolWithPresets(tool, toolConfig.PresetParameters)
				}

				result[serverName][toolName] = toolWithApproval{
					Tool:            tool,
					RequireApproval: toolConfig.RequireApproval,
				}
			}
		}
	}

	return result, nil
}

func modifyToolWithPresets(original domain.Tool, presets map[string]string) domain.Tool {
	modified := original

	// Create new properties map
	newProps := make(map[string]domain.Property)
	newRequired := make([]string, 0)

	// Copy non-preset parameters
	for name, prop := range original.Parameters.Properties {
		if _, isPreset := presets[name]; !isPreset {
			newProps[name] = prop
			// Only include in required if it's not preset
			for _, req := range original.Parameters.Required {
				if req == name {
					newRequired = append(newRequired, name)
				}
			}
		}
	}

	modified.Parameters = domain.Parameters{
		Type:       original.Parameters.Type,
		Properties: newProps,
		Required:   newRequired,
	}

	return modified
}

// PendingFunctionCallError is returned when a function call needs user approval
type PendingFunctionCallError struct {
	Message   *domain.Message
	ToolCalls []llm.ToolCall
}

func (e *PendingFunctionCallError) Error() string {
	return "Tool calls require approval"
}

// ExecuteTools executes a set of tool calls and returns the formatted results
func (a *Agent) ExecuteTools(ctx context.Context, toolCalls []llm.ToolCall) (string, error) {
	// Create channels for collecting results
	type toolResult struct {
		call   llm.ToolCall
		result string
		err    error
	}

	resultChan := make(chan toolResult, len(toolCalls))

	// Execute tools concurrently
	for _, call := range toolCalls {
		go func(tc llm.ToolCall) {
			result, err := a.executeFunction(ctx, tc, a.tools)
			resultChan <- toolResult{
				call:   tc,
				result: result,
				err:    err,
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

	return combinedResults.String(), nil
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

// RejectTools handles tool rejection with an optional message
func (a *Agent) RejectTools(ctx context.Context, messageID uuid.UUID, reason string, opts SendMessageOptions) (*domain.Message, error) {
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

	// Send rejection message
	content := fmt.Sprintf("Function call denied: %s\nPlease suggest an alternative approach.", reason)
	return a.SendMessage(ctx, SendMessageOptions{
		ThreadID:      msg.ThreadID,
		ParentID:      &messageID,
		Content:       content,
		Role:          domain.RoleHuman,
		StreamHandler: opts.StreamHandler,
	})
}

// validateArguments checks if the provided arguments match the tool's schema
func validateArguments(args json.RawMessage, tool toolWithApproval) error {
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

func (a *Agent) executeFunction(ctx context.Context, toolCall llm.ToolCall, tools map[string]map[string]toolWithApproval) (string, error) {
	// Find the tool
	for serverName, serverTools := range tools {
		for toolName, tool := range serverTools {
			if fmt.Sprintf("%s__%s", serverName, toolName) == toolCall.Name {
				// Parse provided arguments
				var providedArgs map[string]interface{}
				if err := json.Unmarshal(toolCall.Arguments, &providedArgs); err != nil {
					return "", fmt.Errorf("failed to parse arguments: %w", err)
				}

				// Check if any parameters were preset
				originalTools := a.mcpClient.GetTools()
				originalTool := originalTools[serverName][toolName]

				// Find preset parameters by comparing schemas
				presetParams := make(map[string]string)
				for paramName := range originalTool.Parameters.Properties {
					if _, exists := tool.Parameters.Properties[paramName]; !exists {
						// Parameter was preset - find its value
						for _, toolsetName := range a.preset.Toolsets {
							if toolset, ok := a.toolsets[toolsetName]; ok {
								if serverConfig, ok := toolset.Servers[serverName]; ok {
									if toolConfig, ok := serverConfig.AllowedTools[toolName]; ok {
										if value, ok := toolConfig.PresetParameters[paramName]; ok {
											presetParams[paramName] = value
										}
									}
								}
							}
						}
					}
				}

				// Merge preset parameters with provided arguments
				mergedArgs := make(map[string]interface{})
				for k, v := range presetParams {
					mergedArgs[k] = v
				}
				for k, v := range providedArgs {
					mergedArgs[k] = v
				}

				// Validate against tool schema
				if err := validateArguments(toolCall.Arguments, tool); err != nil {
					return "", fmt.Errorf("argument validation failed: %w", err)
				}

				// Execute the function
				result, err := a.mcpClient.CallTool(ctx, serverName, toolName, mergedArgs)
				if err != nil {
					return "", fmt.Errorf("function execution failed: %w", err)
				}

				resultBytes, err := json.Marshal(result)
				if err != nil {
					return "", fmt.Errorf("failed to format result: %w", err)
				}

				return string(resultBytes), nil
			}
		}
	}

	return "", fmt.Errorf("tool %s not found", toolCall.Name)
}

type systemMessageOpts struct {
	messageContent string
	history        []domain.Message
}

func (a *Agent) buildSystemMessage(opts systemMessageOpts) (*domain.Message, error) {
	var parts []string

	// 1. Start with preset's system message if it exists
	if a.preset.SystemMessage != "" {
		parts = append(parts, a.preset.SystemMessage)
	}

	// 2. Add explicitly included prompts from preset
	for _, promptName := range a.preset.IncludePrompts {
		if prompt, ok := a.prompts[promptName]; ok {
			parts = append(parts, prompt.Content)
		} else {
			return nil, fmt.Errorf("could not find prompt %s when building system instructions", promptName)
		}
	}

	// 3. Add auto-included prompts and regex-triggered prompts
	messageAndHistory := opts.messageContent
	for _, msg := range opts.history {
		messageAndHistory += "\n" + msg.Content
	}

	for promptName, prompt := range a.prompts {
		// Check auto-include
		if prompt.IncludeInSystemMessage {
			parts = append(parts, prompt.Content)
			continue
		}

		// Check regex trigger if one is set
		if prompt.SystemMessageTrigger != "" {
			matched, err := regexp.MatchString(prompt.SystemMessageTrigger, messageAndHistory)
			if err != nil {
				return nil, fmt.Errorf("failed to evaluate regex trigger for prompt %s: %w", promptName, err)
			}
			if matched {
				parts = append(parts, prompt.Content)
			}
		}
	}

	// 4. Add system messages from active toolsets
	for _, toolsetName := range a.preset.Toolsets {
		if toolset, ok := a.toolsets[toolsetName]; ok && toolset.SystemMessage != "" {
			parts = append(parts, toolset.SystemMessage)
		}
	}

	// 5. Add system messages from MCP servers that have tools in use
	for serverName := range a.tools {
		if server, ok := a.mcpClient.Servers[serverName]; ok && server.SystemMessage != "" {
			parts = append(parts, server.SystemMessage)
		}
	}

	// Join all parts with double newlines
	systemMessage := strings.Join(parts, "\n\n")

	if systemMessage == "" {
		return nil, nil
	}

	return &domain.Message{
		Role:    domain.RoleSystem,
		Content: systemMessage,
	}, nil
}

// SendMessage sends a message through the Agent, handling any function calls
func (a *Agent) SendMessage(ctx context.Context, opts SendMessageOptions) (*domain.Message, error) {
	// Validation
	thread, err := a.repository.GetThread(ctx, opts.ThreadID)
	if err != nil {
		return nil, fmt.Errorf("failed to get thread: %w", err)
	}
	if opts.Role == domain.RoleAssistant {
		return nil, fmt.Errorf("cannot send message with role Assistant")
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
	history, err := a.repository.GetMessages(ctx, thread.ID, opts.ParentID, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get conversation history: %w", err)
	}

	// Build system message
	systemMessage, err := a.buildSystemMessage(systemMessageOpts{
		messageContent: opts.Content,
		history:        history,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to build system message: %w", err)
	}

	// Create message
	userMsg := &domain.Message{
		ThreadID: opts.ThreadID,
		ParentID: opts.ParentID,
		Role:     opts.Role,
		Content:  opts.Content,
	}

	if err := a.repository.AddMessageToThread(ctx, opts.ThreadID, userMsg); err != nil {
		return nil, err
	}

	// Get AI response
	generateOptions := llm.GenerateContentOptions{
		Preset:        a.preset,
		Content:       opts.Content,
		SystemMessage: systemMessage,
		History:       history,
		Tools:         flattenTools(a.tools),
		StreamHandler: opts.StreamHandler,
	}
	aiResponse, err := llm.GenerateContent(
		ctx,
		generateOptions,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate AI response: %w", err)
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
		ModelName: a.preset.Name,
		Provider:  a.preset.Provider,
	}

	if err := a.repository.AddMessageToThread(ctx, opts.ThreadID, aiMsg); err != nil {
		return nil, err
	}

	// Check for function calls in response
	if len(aiResponse.ToolCalls) == 0 {
		return aiMsg, nil
	}

	// Check if any tools require approval
	var toolsNeedingApproval []llm.ToolCall

	for _, call := range aiResponse.ToolCalls {
		// Find tool approval setting
		for serverName, serverTools := range a.tools {
			for toolName, tool := range serverTools {
				if fmt.Sprintf("%s__%s", serverName, toolName) == call.Name {
					if tool.RequireApproval {
						toolsNeedingApproval = append(toolsNeedingApproval, call)
					}
				}
			}
		}
	}

	// If any tools need approval, return error with all of them
	if len(toolsNeedingApproval) > 0 {
		return aiMsg, &PendingFunctionCallError{
			Message:   aiMsg,
			ToolCalls: toolsNeedingApproval,
		}
	}

	// All tools are auto-approved, execute them concurrently
	results, err := a.ExecuteTools(ctx, aiResponse.ToolCalls)
	if err != nil {
		return nil, fmt.Errorf("failed to execute tools: %w", err)
	}

	return a.SendMessage(ctx, SendMessageOptions{
		ThreadID:      opts.ThreadID,
		ParentID:      &aiMsg.ID,
		Content:       results,
		Role:          domain.RoleTool,
		StreamHandler: opts.StreamHandler,
	})
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
