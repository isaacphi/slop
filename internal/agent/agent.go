package agent

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/isaacphi/slop/internal/config"
	"github.com/isaacphi/slop/internal/domain"
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
