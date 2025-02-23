package msg

import (
	"github.com/spf13/cobra"
)

var MsgCmd = &cobra.Command{
	Use:   "msg",
	Short: "Manage messages in conversations",
}
