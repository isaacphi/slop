package msg

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/isaacphi/slop/internal/agent"
	"github.com/isaacphi/slop/internal/config"
	"github.com/isaacphi/slop/internal/mcp"
	messageService "github.com/isaacphi/slop/internal/message"
	"github.com/spf13/cobra"
)

var editCmd = &cobra.Command{
	Use:   "edit [threadID] [messageID] [message]",
	Short: "Create an alternative response to a message",
	Long: `Create an alternative response to a message by starting a new branch from the same parent.
Both threadID and messageID can be partial IDs - they will match the first thread/message that starts with the given ID.`,
	Args: cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Create cancellable context
		ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer cancel()

		// Initialize services
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

		service, err := messageService.InitializeMessageService(cfg)
		if err != nil {
			return err
		}
		mcpClient := mcp.New(cfg.MCPServers)
		agentService := agent.New(service, mcpClient, cfg.Agent)

		// Find thread by partial ID
		thread, err := service.FindThreadByPartialID(ctx, args[0])
		if err != nil {
			return fmt.Errorf("failed to find thread: %w", err)
		}

		// Find message by partial ID within the thread
		targetMessage, err := service.FindMessageByPartialID(ctx, thread.ID, args[1])
		if err != nil {
			return fmt.Errorf("failed to find message: %w", err)
		}

		// Get the message content
		var message string
		if len(args) > 2 {
			message = strings.Join(args[2:], " ")
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

		fmt.Println(targetMessage.ID, targetMessage.Content)

		// Set StreamCallback
		streamCallback := func(chunk []byte) error {
			fmt.Print(string(chunk))
			return nil
		}
		if noStreamFlag {
			streamCallback = nil
		}

		// Send the new message using the parent of the target message as our parent
		sendOptions := messageService.SendMessageOptions{
			ThreadID:       thread.ID,
			ParentID:       targetMessage.ParentID,
			Content:        message,
			StreamCallback: streamCallback,
		}

		// In edit.go RunE function, replace the send logic with:
		if err := sendMessage(ctx, agentService, sendOptions, false); err != nil {
			return err
		}

		// Add followup mode similar to send.go:
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

				if err := sendMessage(ctx, agentService, sendOptions, true); err != nil {
					return err
				}
			}
		}

		return nil
	},
}

func init() {
	editCmd.Flags().BoolVarP(&followupFlag, "followup", "f", false, "Enable followup mode")
	editCmd.Flags().StringVarP(&modelFlag, "model", "m", "", "Specify the model to use")
	editCmd.Flags().BoolVarP(&noStreamFlag, "no-stream", "n", false, "Disable streaming of responses")
	editCmd.Flags().IntVar(&maxTokensFlag, "max-tokens", 0, "Override maximum length")
	editCmd.Flags().Float64Var(&temperatureFlag, "temperature", 0, "Override temperature")
}

func GetEditCommand() *cobra.Command {
	return editCmd
}
