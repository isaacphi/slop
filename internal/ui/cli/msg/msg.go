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
	continueFlag bool
	followupFlag bool
	modelFlag    string
	noStreamFlag bool
)

var MsgCmd = &cobra.Command{
	Use:   "msg [message]",
	Short: "Send messages to an LLM",
	// Args:  cobra.MinimumNArgs(1), // Allow 0 args for pipe input
	RunE: func(cmd *cobra.Command, args []string) error {
		// Create cancellable context
		ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer cancel()

		// Initialize services
		chatService, err := shared.InitializeChatService("")
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
		if continueFlag {
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

		// Send initial message
		if err := sendMessage(ctx, chatService, threadID, message, false); err != nil {
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

				if err := sendMessage(ctx, chatService, threadID, message, true); err != nil {
					return err
				}
			}
		}

		return nil
	},
}

func init() {
	MsgCmd.Flags().BoolVarP(&continueFlag, "continue", "c", false, "Continue the most recent thread")
	MsgCmd.Flags().BoolVarP(&followupFlag, "followup", "f", false, "Enable followup mode")
	MsgCmd.Flags().StringVarP(&modelFlag, "model", "m", "", "Specify the model to use")
	MsgCmd.Flags().BoolVarP(&noStreamFlag, "no-stream", "n", false, "Disable streaming of responses")
}

func sendMessage(ctx context.Context, chatService *service.ChatService, threadID uuid.UUID, message string, isFollowup bool) error {
	if !isFollowup {
		fmt.Printf("You: %s\n", message)
	}
	fmt.Print("Slop: ")

	errCh := make(chan error, 1)

	if noStreamFlag {
		// Use non-streaming version
		go func() {
			resp, err := chatService.SendMessage(ctx, threadID, message)
			if err != nil {
				errCh <- err
				return
			}
			fmt.Print(resp.Content)
			errCh <- nil
		}()
	} else {
		// Use streaming version (default)
		go func() {
			errCh <- chatService.SendMessageStream(ctx, threadID, message, func(chunk string) error {
				fmt.Print(chunk)
				return nil
			})
		}()
	}

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
