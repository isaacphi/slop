package msg

import (
	"github.com/spf13/cobra"
)

var MsgCmd = &cobra.Command{
	Use:   "msg",
	Short: "Send messages",
	Long:  `Send messages to the LLM in various ways.`,
}

func init() {
	MsgCmd.AddCommand(sendCmd)
}
