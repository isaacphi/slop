package msg

import (
	"fmt"
	"github.com/isaacphi/wheel/internal/config"
	"github.com/isaacphi/wheel/internal/llm"
	"github.com/spf13/cobra"
)

var sendCmd = &cobra.Command{
	Use:   "send [message]",
	Short: "Send a single message",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		fmt.Println("Config:", cfg)

		client, err := llm.NewClient(nil)
		if err != nil {
			return fmt.Errorf("failed to create LLM client: %w", err)
		}

		fmt.Printf("You: %s\n", args[0])
		fmt.Print("Assistant: ")

		err = client.ChatStream(cmd.Context(), args[0], func(chunk []byte) error {
			fmt.Print(string(chunk))
			return nil
		})
		if err != nil {
			return fmt.Errorf("chat failed: %w", err)
		}
		fmt.Println()
		return nil
	},
}

func newSendCmd() *cobra.Command {
	return sendCmd
}
