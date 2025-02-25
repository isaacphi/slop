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

type SendMessageOptions struct {
	ThreadID      uuid.UUID
	ParentID      *uuid.UUID
	Content       string
	StreamHandler llm.StreamHandler
	Role          domain.Role
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
		return nil, fmt.Errorf("failed to parse ToolCalls: %w", err)
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
