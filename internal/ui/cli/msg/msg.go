package msg

import (
	"github.com/spf13/cobra"
)

var MsgCmd = &cobra.Command{
	Use:   "msg",
	Short: "Send messages",
	Long:  `Send messages to an LLM.`,
}

func init() {
	MsgCmd.AddCommand(sendCmd)
}
