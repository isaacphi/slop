package msg

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/google/uuid"
	"github.com/isaacphi/slop/internal/agent"
	"github.com/isaacphi/slop/internal/config"
	"github.com/isaacphi/slop/internal/mcp"
	messageService "github.com/isaacphi/slop/internal/message"
	"github.com/spf13/cobra"
)

var (
	continueFlag    bool
	followupFlag    bool
	modelFlag       string
	threadFlag      string
	noStreamFlag    bool
	maxTokensFlag   int
	temperatureFlag float64
)

// Handles function call detection and formatting
func functionCallStreamHandler(originalCallback func([]byte) error) func([]byte) error {
	// Use a closure to encapsulate the state of the streamed function call
	handler := &FunctionCallHandler{}

	return func(chunk []byte) error {
		// Try to detect start of function call if not already in one
		if !handler.inFunctionCall {
			if functionChunk, err := handler.tryParseFunctionChunk(string(chunk)); err == nil {
				handler.inFunctionCall = true
				handler.functionName = functionChunk.Name
				fmt.Printf("\n\n[Requesting tool use: %s]", functionChunk.Name)
				return nil
			}
			return originalCallback(chunk)
		}

		// Accumulate and format function call arguments
		if functionChunk, err := handler.tryParseFunctionChunk(string(chunk)); err == nil {
			handler.argBuffer.WriteString(functionChunk.Arguments)
			fmt.Print(formatPartialJSON(functionChunk.Arguments, handler))
			return nil
		}
		return nil
	}
}

// FunctionCallHandler manages streaming function call state
type FunctionCallHandler struct {
	inFunctionCall bool
	functionName   string
	argBuffer      strings.Builder
	inQuote        bool
	escaped        bool
}

type FunctionCallChunk struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

func (h *FunctionCallHandler) tryParseFunctionChunk(chunk string) (FunctionCallChunk, error) {
	var fcall []struct {
		Function FunctionCallChunk `json:"function"`
	}
	if err := json.Unmarshal([]byte(chunk), &fcall); err == nil {
		return fcall[0].Function, nil
	} else {
		return FunctionCallChunk{}, err
	}
}

// formatPartialJSON formats JSON chunks for display
func formatPartialJSON(partial string, handler *FunctionCallHandler) string {
	// TODO: support multiple tool calls
	var formatted strings.Builder

	for _, char := range partial {
		switch {
		case handler.escaped:
			// Handle escaped character
			formatted.WriteRune(char)
			handler.escaped = false
		case char == '\\':
			// Start of escape sequence
			formatted.WriteRune(char)
			handler.escaped = true
		case char == '"':
			// Toggle quote state
			handler.inQuote = !handler.inQuote
		case char == '{' && !handler.inQuote:
			formatted.WriteRune('\n')
		case char == '}' && !handler.inQuote:
			formatted.WriteRune('\n')
		case char == ',' && !handler.inQuote:
			formatted.WriteRune('\n')
		case char == ' ' && !handler.inQuote:
		case char == ':' && !handler.inQuote:
			formatted.WriteString(": ")
		default:
			formatted.WriteRune(char)
		}
	}
	// return partial
	return formatted.String()
}

var sendCmd = &cobra.Command{
	Use:   "send [message]",
	Short: "Send messages to an LLM",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Create cancellable context
		ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer cancel()

		// Get config
		overrides := &config.RuntimeOverrides{}
		if modelFlag != "" {
			overrides.ActiveModel = &modelFlag
		}
		if maxTokensFlag > 0 {
			overrides.MaxTokens = &maxTokensFlag
		}
		if temperatureFlag > 0 {
			overrides.Temperature = &temperatureFlag
		}
		cfg, err := config.New(overrides)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Initialize services
		service, err := messageService.InitializeMessageService(cfg)
		if err != nil {
			return err
		}
		mcpClient := mcp.New(cfg.MCPServers)
		if err := mcpClient.Initialize(context.Background()); err != nil {
			return fmt.Errorf("failed to initialize MCP client: %w", err)
		}
		defer mcpClient.Shutdown()
		agentService := agent.New(service, mcpClient, cfg.Agent)

		// Get the message content
		var message string
		if len(args) > 0 {
			message = strings.Join(args, " ")
		} else {
			// Check for piped input
			stat, _ := os.Stdin.Stat()
			if (stat.Mode() & os.ModeCharDevice) == 0 {
				bytes, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("failed to read piped input: %w", err)
				}
				message = strings.TrimSpace(string(bytes))
			}
		}

		if message == "" {
			return fmt.Errorf("no message provided")
		}

		// Get thread ID
		var threadID uuid.UUID
		if continueFlag && threadFlag != "" {
			return fmt.Errorf("cannot specify --target and --continue")
		}
		if threadFlag != "" {
			thread, err := service.FindThreadByPartialID(ctx, threadFlag)
			if err != nil {
				return err
			}
			threadID = thread.ID
		} else if continueFlag {
			thread, err := service.GetActiveThread(ctx)
			if err != nil {
				return err
			}
			threadID = thread.ID
		} else {
			// Create new thread
			thread, err := service.NewThread(ctx)
			if err != nil {
				return fmt.Errorf("failed to create thread: %w", err)
			}
			threadID = thread.ID
		}

		// Set StreamCallback
		streamCallback := func(chunk []byte) error {
			fmt.Print(string(chunk))
			return nil
		}
		if noStreamFlag {
			streamCallback = nil
		}
		sendOptions := messageService.SendMessageOptions{
			ThreadID:       threadID,
			Content:        message,
			StreamCallback: streamCallback,
		}

		// Send initial message
		if err := sendMessage(ctx, agentService, sendOptions, false); err != nil {
			return err
		}

		// Handle followup mode
		if followupFlag {
			reader := bufio.NewReader(os.Stdin)
			for {
				fmt.Print("\nYou: ")
				message, err := reader.ReadString('\n')
				if err == io.EOF {
					break
				}
				if err != nil {
					return fmt.Errorf("failed to read input: %w", err)
				}

				message = strings.TrimSpace(message)
				if message == "" {
					continue
				}

				sendOptions.Content = message
				if err := sendMessage(ctx, agentService, sendOptions, true); err != nil {
					return err
				}
			}
		}

		return nil
	},
}

func sendMessage(ctx context.Context, agentService *agent.Agent, opts messageService.SendMessageOptions, isFollowup bool) error {
	if !isFollowup {
		fmt.Printf("You: %s\n", opts.Content)
	}
	fmt.Print("Slop: ")

	if !noStreamFlag {
		origCallback := opts.StreamCallback
		opts.StreamCallback = functionCallStreamHandler(origCallback)
	}

	errCh := make(chan error, 1)
	go func() {
		resp, err := agentService.SendMessage(ctx, opts)
		if err != nil {
			errCh <- err
			return
		}
		if noStreamFlag {
			fmt.Print(resp.Content)
		}
		// note: gemini does not stream tool use (is this an issue with langchaingo?)
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

	fmt.Println()
	return nil
}

func init() {
	sendCmd.Flags().StringVarP(&threadFlag, "thread", "t", "", "Continue target thread")
	sendCmd.Flags().BoolVarP(&continueFlag, "continue", "c", false, "Continue the most recent thread")
	sendCmd.Flags().BoolVarP(&followupFlag, "followup", "f", false, "Enable followup mode")
	sendCmd.Flags().StringVarP(&modelFlag, "model", "m", "", "Specify the model to use")
	sendCmd.Flags().BoolVarP(&noStreamFlag, "no-stream", "n", false, "Disable streaming of responses")
	sendCmd.Flags().IntVar(&maxTokensFlag, "max-tokens", 0, "Override maximum length")
	sendCmd.Flags().Float64Var(&temperatureFlag, "temperature", 0, "Override temperature")
}
