package mcp

import (
	"context"
	"fmt"

	"github.com/isaacphi/slop/internal/app"
	"github.com/isaacphi/slop/internal/mcp"
	"github.com/spf13/cobra"
)

var (
	MCPCmd = &cobra.Command{
		Use:   "mcp",
		Short: "Display MCP tools information",
		Long:  "Initialize MCP servers and display information about available tools",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load configuration
			cfg := app.Get().Config

			// Create and initialize MCP client
			client := mcp.New(cfg.MCPServers)
			if err := client.Initialize(context.Background()); err != nil {
				return fmt.Errorf("failed to initialize MCP client: %w", err)
			}
			defer client.Shutdown()

			client.PrintTools()

			return nil
		},
	}
)
