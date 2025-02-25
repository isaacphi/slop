package msg

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/isaacphi/slop/internal/agent"
	"github.com/isaacphi/slop/internal/appState"
	"github.com/isaacphi/slop/internal/domain"
	"github.com/isaacphi/slop/internal/events"
	"github.com/isaacphi/slop/internal/llm"
	"github.com/isaacphi/slop/internal/mcp"
	"github.com/isaacphi/slop/internal/repository/sqlite"
	"github.com/spf13/cobra"
)

var (
	continueFlag    bool
	modelFlag       string
	threadFlag      string
	parentFlag      string
	noStreamFlag    bool
	maxTokensFlag   int
	temperatureFlag float64
	approveFlag     bool
	rejectFlag      bool
)

var sendCmd = &cobra.Command{
	Use:   "send [message]",
	Short: "Send messages to an LLM",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		cfg := appState.Get().Config

		// Initialize repository
		repo, err := sqlite.Initialize(cfg.DBPath)
		if err != nil {
			return fmt.Errorf("failed to initialize repository: %w", err)
		}

		// Initialize MCP client
		mcpClient := mcp.New(cfg.MCPServers)
		if err := mcpClient.Initialize(context.Background()); err != nil {
			return fmt.Errorf("failed to initialize MCP client: %w", err)
		}
		defer mcpClient.Shutdown()

		// Get model configuration
		preset := cfg.Presets[cfg.DefaultPreset]
		if modelFlag != "" {
			var ok bool
			preset, ok = cfg.Presets[modelFlag]
			if !ok {
				return fmt.Errorf("model %s not found in configuration", modelFlag)
			}
		}
		if maxTokensFlag > 0 {
			preset.MaxTokens = maxTokensFlag
		}
		if temperatureFlag > 0 {
			preset.Temperature = temperatureFlag
		}

		// Initialize Agent
		agentService, err := agent.New(repo, mcpClient, preset, cfg.Toolsets, cfg.Prompts)
		if err != nil {
			return fmt.Errorf("could not initialize MCP agent: %w", err)
		}

		// Check for conflicting flags
		if continueFlag && threadFlag != "" {
			return fmt.Errorf("cannot specify --thread and --continue")
		}

		if approveFlag && rejectFlag {
			return fmt.Errorf("cannot specify both --approve and --reject")
		}

		// Get the message content
		var messageContent string
		if len(args) > 0 {
			messageContent = strings.Join(args, " ")
		} else {
			// Check for piped input
			stat, _ := os.Stdin.Stat()
			if (stat.Mode() & os.ModeCharDevice) == 0 {
				bytes, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("failed to read piped input: %w", err)
				}
				messageContent = strings.TrimSpace(string(bytes))
			}
		}

		// Get thread ID
		var threadID uuid.UUID
		var msg *domain.Message

		// Handle parent flag case (for tool approval or alternative responses)
		if parentFlag != "" {
			if threadFlag == "" {
				return fmt.Errorf("--parent requires --thread to be specified")
			}

			// Find thread
			thread, err := repo.GetThreadByPartialID(ctx, threadFlag)
			if err != nil {
				return fmt.Errorf("failed to find thread: %w", err)
			}
			threadID = thread.ID

			// Find parent message
			parentMsg, err := repo.FindMessageByPartialID(ctx, threadID, parentFlag)
			if err != nil {
				return fmt.Errorf("failed to find parent message: %w", err)
			}

			// Check if parent is an assistant message with tool calls
			if parentMsg.Role == domain.RoleAssistant && parentMsg.ToolCalls != "" {
				// Tool approval/rejection flow
				if approveFlag {
					// For approval, we send the assistant message directly
					// Empty message is valid for approvals
					if messageContent != "" {
						return fmt.Errorf("message content not allowed with --approve")
					}
					msg = parentMsg
				} else if rejectFlag {
					// For rejection, create a human message with optional reason
					msg = &domain.Message{
						ThreadID: threadID,
						ParentID: &parentMsg.ID,
						Role:     domain.RoleHuman,
						Content:  fmt.Sprintf("Tool call rejected: %s", messageContent),
					}
				} else {
					return fmt.Errorf("parent message has pending tool calls; must use --approve or --reject")
				}
			} else {
				// Standard alternative response flow
				// Use the parent's parent as our parent (same as edit command)
				if messageContent == "" {
					return fmt.Errorf("no message provided")
				}
				msg = &domain.Message{
					ThreadID: threadID,
					ParentID: &parentMsg.ID,
					Role:     domain.RoleHuman,
					Content:  messageContent,
				}
			}
		} else {
			// Check if we're continuing a thread
			if continueFlag {
				thread, err := repo.GetMostRecentThread(ctx)
				if err != nil {
					return err
				}
				threadID = thread.ID

				// Get the most recent message to check for pending tool calls
				messages, err := repo.GetMessages(ctx, threadID, nil, false)
				if err != nil {
					return fmt.Errorf("failed to get thread messages: %w", err)
				}

				if len(messages) > 0 {
					lastMsg := messages[len(messages)-1]

					// Check if last message has pending tool calls
					if lastMsg.Role == domain.RoleAssistant && lastMsg.ToolCalls != "" {
						if approveFlag {
							// For approval, we send the assistant message directly
							if messageContent != "" {
								return fmt.Errorf("message content not allowed with --approve")
							}
							msg = &lastMsg
						} else if rejectFlag {
							// For rejection, create a human message with optional reason
							msg = &domain.Message{
								ThreadID: threadID,
								ParentID: &lastMsg.ID,
								Role:     domain.RoleHuman,
								Content:  fmt.Sprintf("Tool call rejected: %s", messageContent),
							}
						} else {
							return fmt.Errorf("last message has pending tool calls; must use --approve or --reject")
						}
					} else {
						// Normal continuation
						if messageContent == "" {
							return fmt.Errorf("no message provided")
						}
						parentID := getLastUserMessageID(messages)
						msg = &domain.Message{
							ThreadID: threadID,
							ParentID: parentID,
							Role:     domain.RoleHuman,
							Content:  messageContent,
						}
					}
				} else {
					// Empty thread case
					if messageContent == "" {
						return fmt.Errorf("no message provided")
					}
					msg = &domain.Message{
						ThreadID: threadID,
						Role:     domain.RoleHuman,
						Content:  messageContent,
					}
				}
			} else if threadFlag != "" {
				// Continuing specific thread
				thread, err := repo.GetThreadByPartialID(ctx, threadFlag)
				if err != nil {
					return err
				}
				threadID = thread.ID

				// Get the most recent messages
				messages, err := repo.GetMessages(ctx, threadID, nil, false)
				if err != nil {
					return fmt.Errorf("failed to get thread messages: %w", err)
				}

				if len(messages) > 0 {
					// Check if last message has pending tool calls
					lastMsg := messages[len(messages)-1]
					if lastMsg.Role == domain.RoleAssistant && lastMsg.ToolCalls != "" {
						if approveFlag {
							// For approval, we send the assistant message directly
							if messageContent != "" {
								return fmt.Errorf("message content not allowed with --approve")
							}
							msg = &lastMsg
						} else if rejectFlag {
							// For rejection, create a human message with optional reason
							msg = &domain.Message{
								ThreadID: threadID,
								ParentID: &lastMsg.ID,
								Role:     domain.RoleHuman,
								Content:  fmt.Sprintf("Tool call rejected: %s", messageContent),
							}
						} else {
							return fmt.Errorf("last message has pending tool calls; must use --approve or --reject")
						}
					} else {
						// Normal continuation
						if messageContent == "" {
							return fmt.Errorf("no message provided")
						}
						parentID := getLastUserMessageID(messages)
						msg = &domain.Message{
							ThreadID: threadID,
							ParentID: parentID,
							Role:     domain.RoleHuman,
							Content:  messageContent,
						}
					}
				} else {
					// Empty thread case
					if messageContent == "" {
						return fmt.Errorf("no message provided")
					}
					msg = &domain.Message{
						ThreadID: threadID,
						Role:     domain.RoleHuman,
						Content:  messageContent,
					}
				}
			} else {
				// Create new thread
				if messageContent == "" {
					return fmt.Errorf("no message provided")
				}

				thread := &domain.Thread{}
				if err := repo.CreateThread(ctx, thread); err != nil {
					return fmt.Errorf("failed to create thread: %w", err)
				}
				threadID = thread.ID

				msg = &domain.Message{
					ThreadID: threadID,
					Role:     domain.RoleHuman,
					Content:  messageContent,
				}
			}
		}

		// Send the message
		if err := sendMessage(ctx, agentService, msg); err != nil {
			return err
		}

		return nil
	},
}

// getLastUserMessageID returns the ID of the last human message in the thread
// to be used as the parent ID for new messages
func getLastUserMessageID(messages []domain.Message) *uuid.UUID {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == domain.RoleHuman {
			return &messages[i].ID
		}
	}
	return nil
}

var sendCmdOld = &cobra.Command{
	Use:   "send [message]",
	Short: "Send messages to an LLM",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		cfg := appState.Get().Config

		// Initialize repository
		repo, err := sqlite.Initialize(cfg.DBPath)
		if err != nil {
			return fmt.Errorf("failed to initialize repository: %w", err)
		}

		// Initialize MCP client
		mcpClient := mcp.New(cfg.MCPServers)
		if err := mcpClient.Initialize(context.Background()); err != nil {
			return fmt.Errorf("failed to initialize MCP client: %w", err)
		}
		defer mcpClient.Shutdown()

		// Get model configuration
		preset := cfg.Presets[cfg.DefaultPreset]
		if modelFlag != "" {
			var ok bool
			preset, ok = cfg.Presets[modelFlag]
			if !ok {
				return fmt.Errorf("model %s not found in configuration", modelFlag)
			}
		}
		if maxTokensFlag > 0 {
			preset.MaxTokens = maxTokensFlag
		}
		if temperatureFlag > 0 {
			preset.Temperature = temperatureFlag
		}

		// Initialize Agent
		agentService, err := agent.New(repo, mcpClient, preset, cfg.Toolsets, cfg.Prompts)
		if err != nil {
			return fmt.Errorf("could not initialize MCP agent: %w", err)
		}

		// Get the initialMessage content
		var initialMessage string
		if len(args) > 0 {
			initialMessage = strings.Join(args, " ")
		} else {
			// Check for piped input
			stat, _ := os.Stdin.Stat()
			if (stat.Mode() & os.ModeCharDevice) == 0 {
				bytes, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("failed to read piped input: %w", err)
				}
				initialMessage = strings.TrimSpace(string(bytes))
			}
		}

		// TODO: an empty message is valid for tool approvals, so move this check later
		// only if this is not a tool approval request
		if initialMessage == "" {
			return fmt.Errorf("no message provided")
		}

		// Get thread ID and parent message if specified
		var threadID uuid.UUID
		var parentID *uuid.UUID

		if continueFlag && threadFlag != "" {
			return fmt.Errorf("cannot specify --thread and --continue")
		}

		// Handle parent flag case
		if parentFlag != "" {
			// TODO: If the parent message is an assistant message with tool calls, then there must be
			// either a new --approve or --reject (but not both) flag set. We then proceed with the approval
			// or rejection. In the case of rejection, the message is optional. There can be no message with approval
			// NOTE: this should also be possible with the --continue flag. I.e. if you continue after a
			// pending tool call, it MUST have either --reject or --approve
			if threadFlag == "" {
				return fmt.Errorf("--parent requires --thread to be specified")
			}
			// Find thread
			thread, err := repo.GetThreadByPartialID(ctx, threadFlag)
			if err != nil {
				return fmt.Errorf("failed to find thread: %w", err)
			}
			threadID = thread.ID

			// Find parent message
			parentMsg, err := repo.FindMessageByPartialID(ctx, threadID, parentFlag)
			if err != nil {
				return fmt.Errorf("failed to find parent message: %w", err)
			}
			// Use the parent's parent as our parent (same as edit command)
			parentID = &parentMsg.ID
		} else {
			// Regular send command flow
			if threadFlag != "" {
				thread, err := repo.GetThreadByPartialID(ctx, threadFlag)
				if err != nil {
					return err
				}
				threadID = thread.ID
			} else if continueFlag {
				thread, err := repo.GetMostRecentThread(ctx)
				if err != nil {
					return err
				}
				threadID = thread.ID
			} else {
				// Create new thread
				thread := &domain.Thread{}
				if err := repo.CreateThread(ctx, thread); err != nil {
					return fmt.Errorf("failed to create thread: %w", err)
				}
				threadID = thread.ID
			}
		}

		sendOptions := agent.SendMessageOptions{
			ThreadID: threadID,
			ParentID: parentID,
			Content:  initialMessage,
		}

		// Send initial message
		if err := sendMessageOld(ctx, agentService, sendOptions); err != nil {
			return err
		}

		return nil
	},
}

func sendMessage(ctx context.Context, agentService *agent.Agent, msg *domain.Message) error {
	// Start the stream with the message
	stream := agentService.SendMessageStream(ctx, msg)

	var pendingMessage *domain.Message
	var pendingToolCallsList []llm.ToolCall

	// Process events from the stream
	for {
		select {
		case <-ctx.Done():
			fmt.Println("\nRequest cancelled")
			return ctx.Err()

		case event, ok := <-stream.Events:
			if !ok {
				// Stream closed without any pending tool calls
				fmt.Println()
				return nil
			}

			switch e := event.(type) {
			case *llm.TextEvent:
				fmt.Print(e.Content)

			case *llm.ToolCallEvent:
				// For the CLI, we might want to indicate that a tool call is happening
				fmt.Printf("\n[Tool call: %s]\n", e.Name)

			case *llm.MessageCompleteEvent:
				// The message is complete with all metadata
				// This is where we'd handle any post-message processing if needed

			case *agent.ToolApprovalRequestEvent:
				pendingMessage = e.Message
				pendingToolCallsList = e.ToolCalls

				// Handle tool approvals right when we get the event
				return handleToolApproval(ctx, agentService, pendingMessage, pendingToolCallsList)

			case *agent.ToolResultEvent:
				fmt.Printf("\nTool %s result: %s\n", e.Name, e.Result)

			case *agent.NewMessageEvent:
				// Update thread state if needed

			case *events.ErrorEvent:
				return e.Error
			}

		case <-stream.Done:
			return nil
		}
	}
}

// Helper function to handle tool approval
func handleToolApproval(ctx context.Context, agentService *agent.Agent, message *domain.Message, toolCalls []llm.ToolCall) error {
	// Print tool calls details
	fmt.Printf("\nPending tool calls:\n")
	for _, call := range toolCalls {
		fmt.Printf("\nName: %s\nArguments: %s\n", call.Name, string(call.Arguments))
	}

	// Prompt for approval
	fmt.Print("\nApprove tool execution? [y/N] ")
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read approval: %w", err)
	}

	response = strings.TrimSpace(strings.ToLower(response))
	if response == "y" || response == "yes" {
		fmt.Println()
		// Execute tools by calling SendMessageStream with the assistant message
		// This is considered an approval
		stream := agentService.SendMessageStream(ctx, message)

		// Process the results
		for {
			select {
			case <-ctx.Done():
				fmt.Println("\nRequest cancelled")
				return ctx.Err()

			case event, ok := <-stream.Events:
				if !ok {
					// Stream closed
					fmt.Println()
					return nil
				}

				switch e := event.(type) {
				case *llm.TextEvent:
					fmt.Print(e.Content)

				case *llm.ToolCallEvent:
					fmt.Printf("\n[Tool call: %s]\n", e.Name)

				case *agent.ToolResultEvent:
					fmt.Printf("\nTool %s result: %s\n", e.Name, e.Result)

				case *agent.ToolApprovalRequestEvent:
					// Handle nested tool approvals
					return handleToolApproval(ctx, agentService, e.Message, e.ToolCalls)

				case *events.ErrorEvent:
					return e.Error
				}

			case <-stream.Done:
				return nil
			}
		}
	} else {
		// Optional rejection reason
		fmt.Print("Enter rejection reason (optional, press Enter to skip): ")
		reason, err := reader.ReadString('\n')
		fmt.Println()

		if err != nil {
			return fmt.Errorf("failed to read reason: %w", err)
		}
		reason = strings.TrimSpace(reason)

		// Create a tool rejection message
		rejectionMsg := &domain.Message{
			ThreadID: message.ThreadID,
			ParentID: &message.ID,
			Role:     domain.RoleHuman,
			Content:  fmt.Sprintf("Tool call rejected: %s", reason),
		}

		// Send the rejection message
		stream := agentService.SendMessageStream(ctx, rejectionMsg)

		// Process the results
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()

			case event, ok := <-stream.Events:
				if !ok {
					// Stream closed
					fmt.Println()
					return nil
				}

				switch e := event.(type) {
				case *llm.TextEvent:
					fmt.Print(e.Content)

				case *llm.ToolCallEvent:
					fmt.Printf("\n[Tool call: %s]\n", e.Name)

				case *agent.ToolApprovalRequestEvent:
					// Handle nested tool approvals
					return handleToolApproval(ctx, agentService, e.Message, e.ToolCalls)

				case *events.ErrorEvent:
					return e.Error
				}

			case <-stream.Done:
				return nil
			}
		}
	}
}

// TODO: sendMessage should take a domain.Message as input
func sendMessageOld(ctx context.Context, agentService *agent.Agent, opts agent.SendMessageOptions) error {
	// stream := agentService.SendMessageStream(ctx, opts)
	stream := agentService.SendMessageStream(ctx, &domain.Message{})

	var pendingMessage *domain.Message
	var pendingToolCallsList []llm.ToolCall

	// Process events from the stream
	for {
		select {
		case <-ctx.Done():
			fmt.Println("\nRequest cancelled")
			return ctx.Err()

		case event, ok := <-stream.Events:
			if !ok {
				// Stream closed without any pending tool calls
				fmt.Println()
				return nil
			}

			switch e := event.(type) {
			case *llm.TextEvent:
				fmt.Print(e.Content)

			case *llm.ToolCallEvent:
				// For the CLI, we might want to indicate that a tool call is happening
				fmt.Printf("\n[Tool call: %s]\n", e.Name)

			case *llm.MessageCompleteEvent:
				// The message is complete with all metadata
				// This is where we'd handle any post-message processing if needed

			case *agent.ToolApprovalRequestEvent:
				pendingMessage = e.Message
				pendingToolCallsList = e.ToolCalls

				// Handle tool approvals right when we get the event
				return handleToolApproval(ctx, agentService, pendingMessage, pendingToolCallsList)

			case *agent.ToolResultEvent:
				fmt.Printf("\nTool %s result: %s\n", e.Name, e.Result)

			case *agent.NewMessageEvent:
				// Update thread state if needed

			case *events.ErrorEvent:
				return e.Error
			}

		case <-stream.Done:
			return nil
		}
	}
}

// Helper function to handle tool approval
func handleToolApprovalOld(ctx context.Context, agentService *agent.Agent, message *domain.Message, toolCalls []llm.ToolCall) error {
	// Print tool calls details
	fmt.Printf("\nPending tool calls:\n")
	for _, call := range toolCalls {
		fmt.Printf("\nName: %s\nArguments: %s\n", call.Name, string(call.Arguments))
	}

	// Prompt for approval
	fmt.Print("\nApprove tool execution? [y/N] ")
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read approval: %w", err)
	}

	response = strings.TrimSpace(strings.ToLower(response))
	if response == "y" || response == "yes" {
		fmt.Println()
		// Execute tools using the existing ApproveAndExecuteTools method
		// TODO: this should use SendMessageStream
		_, err := agentService.ApproveAndExecuteTools(ctx, message.ID, agent.SendMessageOptions{
			// Pass the stream handler if you want to stream the tool execution result
		})
		return err
	} else {
		// Optional rejection reason
		fmt.Print("Enter rejection reason (optional, press Enter to skip): ")
		reason, err := reader.ReadString('\n')
		fmt.Println()

		if err != nil {
			return fmt.Errorf("failed to read reason: %w", err)
		}
		reason = strings.TrimSpace(reason)

		// Use DenyFunctionCall for rejection
		// TODO: This should use SendMessageStream
		_, err = agentService.RejectTools(ctx, message.ID, reason)
		return err
	}
}

// Handles function call detection and formatting
type CLIStreamHandler struct {
	originalCallback func([]byte) error
	inQuote          bool
	escaped          bool
	indentLevel      int
	inArray          bool
	isValue          bool // Tracks if we're after a colon to handle YAML formatting
}

func (h *CLIStreamHandler) HandleTextChunk(chunk []byte) error {
	return h.originalCallback(chunk)
}

func (h *CLIStreamHandler) HandleMessageDone() {
	h.inQuote = false
	h.escaped = false
	fmt.Print("\n\n")
}

func (h *CLIStreamHandler) HandleFunctionCallStart(id, name string) error {
	fmt.Printf("\n\n[Requesting tool use: %s]", name)
	return nil
}

func (h *CLIStreamHandler) HandleFunctionCallChunk(chunk llm.FunctionCallChunk) error {
	fmt.Print(h.formatJSON(chunk.ArgumentsJson))
	return nil
}

func (h *CLIStreamHandler) formatJSON(data string) string {
	// I will later move this to llm.go
	var formatted strings.Builder

	writeIndent := func() {
		for i := 0; i < h.indentLevel; i++ {
			formatted.WriteString("  ")
		}
	}

	for _, char := range data {
		switch {
		case h.escaped:
			formatted.WriteRune(char)
			h.escaped = false

		case char == '\\':
			formatted.WriteRune(char)
			h.escaped = true

		case char == '"':
			// For YAML, we generally don't need the quotes unless there's special characters
			if !h.inQuote {
				// Starting a quote - don't write it
				h.inQuote = true
			} else {
				// Ending a quote - don't write it
				h.inQuote = false
			}

		case char == '[' && !h.inQuote:
			h.inArray = true
			h.indentLevel++
			formatted.WriteString("\n")
			writeIndent()
			formatted.WriteString("- ")

		case char == ']' && !h.inQuote:
			h.indentLevel--
			h.inArray = false

		case char == '{' && !h.inQuote:
			if h.inArray {
				writeIndent()
			}
			h.indentLevel++
			formatted.WriteString("\n")
			writeIndent()

		case char == '}' && !h.inQuote:
			h.indentLevel--
			h.isValue = false

		case char == ',' && !h.inQuote:
			h.isValue = false
			formatted.WriteString("\n")
			if h.inArray {
				writeIndent()
				formatted.WriteString("- ")
			} else {
				writeIndent()
			}

		case char == ':' && !h.inQuote:
			h.isValue = true
			formatted.WriteString(": ")

		case char == ' ' && !h.inQuote:
			// Only keep spaces that are part of actual values
			if h.isValue {
				formatted.WriteRune(char)
			}

		default:
			if h.inArray && !h.isValue && !h.inQuote {
				// If we're starting a new array element
				formatted.WriteString("- ")
				h.isValue = true
			}
			formatted.WriteRune(char)
		}
	}
	return formatted.String()
}

func init() {
	sendCmd.Flags().StringVarP(&threadFlag, "thread", "t", "", "Continue target thread")
	sendCmd.Flags().StringVarP(&parentFlag, "parent", "p", "", "Create alternative response by using specified message's parent")
	sendCmd.Flags().BoolVarP(&continueFlag, "continue", "c", false, "Continue the most recent thread")
	sendCmd.Flags().StringVarP(&modelFlag, "model", "m", "", "Specify the model to use")
	sendCmd.Flags().BoolVarP(&noStreamFlag, "no-stream", "n", false, "Disable streaming of responses")
	sendCmd.Flags().IntVar(&maxTokensFlag, "max-tokens", 0, "Override maximum length")
	sendCmd.Flags().Float64Var(&temperatureFlag, "temperature", 0, "Override temperature")
	sendCmd.Flags().BoolVarP(&approveFlag, "approve", "a", false, "Approve pending tool calls")
	sendCmd.Flags().BoolVarP(&rejectFlag, "reject", "r", false, "Reject pending tool calls")
	MsgCmd.AddCommand(sendCmd)
}
