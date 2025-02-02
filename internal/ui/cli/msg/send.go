package msg

import (
	"bufio"
	"context"
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

				if err := sendMessage(ctx, chatService, sendOptions, true); err != nil {
					return err
				}
			}
		}

		return nil
	},
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

// In send.go
func sendMessage(ctx context.Context, chatService *service.ChatService, opts service.SendMessageOptions, isFollowup bool) error {
	if !isFollowup {
		fmt.Printf("You: %s\n", opts.Content)
	}
	fmt.Print("Slop: ")

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
