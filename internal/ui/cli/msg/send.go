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
	"github.com/isaacphi/slop/internal/service"
	"github.com/isaacphi/slop/internal/shared"
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
func newFunctionCallStreamHandler(originalCallback func(string) error) func(string) error {
	handler := &FunctionCallHandler{}

	return func(chunk string) error {
		// Try to detect start of function call if not already in one
		if !handler.inFunctionCall {
			if functionName := handler.tryParseFunctionName(chunk); functionName != "" {
				handler.inFunctionCall = true
				handler.functionName = functionName
				fmt.Printf("\n\n[Requesting tool use: %s]", functionName)
				return nil
			}
			return originalCallback(chunk)
		}

		// Accumulate and format function call arguments
		if argumentDiff := handler.tryParseFunctionChunk(chunk); argumentDiff != "" {
			handler.argBuffer.WriteString(argumentDiff)
			// fmt.Print(formatPartialJSON(handler.argBuffer.String()))
			fmt.Print(formatPartialJSON(argumentDiff, handler))
			// fmt.Println(handler.argBuffer.String())
			return nil
		}

		return nil
	}
}

var fcall []struct {
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

func (h *FunctionCallHandler) tryParseFunctionName(chunk string) string {
	if err := json.Unmarshal([]byte(chunk), &fcall); err == nil && fcall[0].Function.Name != "" {
		return fcall[0].Function.Name
	}
	return ""
}

func (h *FunctionCallHandler) tryParseFunctionChunk(chunk string) string {
	if err := json.Unmarshal([]byte(chunk), &fcall); err == nil && fcall[0].Function.Arguments != "" {
		return fcall[0].Function.Arguments
	}
	return ""
}

// FunctionCallHandler manages streaming function call state
type FunctionCallHandler struct {
	inFunctionCall bool
	functionName   string
	argBuffer      strings.Builder
	inQuote        bool
	escaped        bool
}

// formatPartialJSON formats JSON chunks for display
func formatPartialJSON(partial string, handler *FunctionCallHandler) string {
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

		// Initialize services
		opts := &shared.ServiceOptions{
			Model:       modelFlag,
			MaxTokens:   maxTokensFlag,
			Temperature: temperatureFlag,
		}
		chatService, err := shared.InitializeChatService(opts)
		if err != nil {
			return err
		}

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
			thread, err := chatService.FindThreadByPartialID(ctx, threadFlag)
			if err != nil {
				return err
			}
			threadID = thread.ID
		} else if continueFlag {
			thread, err := chatService.GetActiveThread(ctx)
			if err != nil {
				return err
			}
			threadID = thread.ID
		} else {
			// Create new thread
			thread, err := chatService.NewThread(ctx)
			if err != nil {
				return fmt.Errorf("failed to create thread: %w", err)
			}
			threadID = thread.ID
		}

		sendOptions := service.SendMessageOptions{
			ThreadID: threadID,
			Content:  message,
			Stream:   !noStreamFlag,
			StreamCallback: func(chunk string) error {
				fmt.Print(chunk)
				return nil
			},
		}

		// Send initial message
		if err := sendMessage(ctx, chatService, sendOptions, false); err != nil {
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
				if err := sendMessage(ctx, chatService, sendOptions, true); err != nil {
					return err
				}
			}
		}

		return nil
	},
}

func sendMessage(ctx context.Context, chatService *service.ChatService, opts service.SendMessageOptions, isFollowup bool) error {
	if !isFollowup {
		fmt.Printf("You: %s\n", opts.Content)
	}
	fmt.Print("Slop: ")

	origCallback := opts.StreamCallback
	opts.StreamCallback = newFunctionCallStreamHandler(origCallback)

	errCh := make(chan error, 1)
	go func() {
		resp, err := chatService.SendMessage(ctx, opts)
		if err != nil {
			errCh <- err
			return
		}
		if !opts.Stream {
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

