package msg

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/isaacphi/slop/internal/agent"
	"github.com/isaacphi/slop/internal/appState"
	"github.com/isaacphi/slop/internal/domain"
	"github.com/isaacphi/slop/internal/llm"
	"github.com/isaacphi/slop/internal/mcp"
	"github.com/isaacphi/slop/internal/repository/sqlite"
	"github.com/spf13/cobra"
)

var (
	continueFlag    bool
	followupFlag    bool
	modelFlag       string
	threadFlag      string
	parentFlag      string
	noStreamFlag    bool
	maxTokensFlag   int
	temperatureFlag float64
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
		if err := sendMessage(ctx, agentService, sendOptions); err != nil {
			return err
		}

		// Handle followup mode
		if followupFlag {
			reader := bufio.NewReader(os.Stdin)
			for {
				fmt.Print("> ")
				followupMessage, err := reader.ReadString('\n')
				fmt.Println()
				if err == io.EOF {
					break
				}
				if err != nil {
					return fmt.Errorf("failed to read input: %w", err)
				}

				followupMessage = strings.TrimSpace(followupMessage)
				if followupMessage == "" {
					continue
				}

				sendOptions.Content = followupMessage
				if err := sendMessage(ctx, agentService, sendOptions); err != nil {
					return err
				}
			}
		}

		return nil
	},
}

func sendMessage(ctx context.Context, agentService *agent.Agent, opts agent.SendMessageOptions) error {
	if !noStreamFlag {
		opts.StreamHandler = &CLIStreamHandler{originalCallback: func(chunk []byte) error {
			fmt.Print(string(chunk))
			return nil
		}}
	}

	errCh := make(chan error, 1)
	var respMsg *domain.Message
	pendingToolCalls := false

	go func() {
		resp, err := agentService.SendMessage(ctx, opts)
		respMsg = resp
		if err != nil {
			if _, ok := err.(*agent.PendingFunctionCallError); ok {
				// Don't send to error channel - we'll handle this case
				pendingToolCalls = true
				errCh <- nil
				return
			}
			errCh <- err
			return
		}
		if noStreamFlag {
			fmt.Print(resp.Content)
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		fmt.Println("\nRequest cancelled")
		return ctx.Err()
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("failed to send message: %w", err)
		}
	}

	// Check if we have pending tool calls to handle
	if pendingToolCalls && respMsg != nil && respMsg.ToolCalls != "" {
		var toolCalls []llm.ToolCall
		if err := json.Unmarshal([]byte(respMsg.ToolCalls), &toolCalls); err != nil {
			return fmt.Errorf("failed to parse tool calls: %w", err)
		}

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
			// Execute tools
			_, err := agentService.ApproveAndExecuteTools(ctx, respMsg.ID, agent.SendMessageOptions{
				StreamHandler: opts.StreamHandler,
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
			_, err = agentService.RejectTools(ctx, respMsg.ID, reason, agent.SendMessageOptions{
				StreamHandler: opts.StreamHandler,
			})
			return err
		}
	}

	fmt.Println()
	return nil
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
	sendCmd.Flags().BoolVarP(&followupFlag, "followup", "f", false, "Enable followup mode")
	sendCmd.Flags().StringVarP(&modelFlag, "model", "m", "", "Specify the model to use")
	sendCmd.Flags().BoolVarP(&noStreamFlag, "no-stream", "n", false, "Disable streaming of responses")
	sendCmd.Flags().IntVar(&maxTokensFlag, "max-tokens", 0, "Override maximum length")
	sendCmd.Flags().Float64Var(&temperatureFlag, "temperature", 0, "Override temperature")
	MsgCmd.AddCommand(sendCmd)
}
