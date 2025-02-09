package mcp

import (
	"context"
	"fmt"
	"sort"

	"github.com/isaacphi/slop/internal/config"
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
			cfg, err := config.New()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Create and initialize MCP client
			client := mcp.New(cfg.MCPServers)
			if err := client.Initialize(context.Background()); err != nil {
				return fmt.Errorf("failed to initialize MCP client: %w", err)
			}
			defer client.Shutdown()

			// Get all available tools
			tools := client.GetTools()

			// Create sorted list of tool names for consistent output
			var toolNames []string
			for name := range tools {
				toolNames = append(toolNames, name)
			}
			sort.Strings(toolNames)

			// Group tools by server
			serverTools := make(map[string]map[string]config.Tool)
			for name, tool := range tools {
				serverName := tool.Name
				if serverTools[serverName] == nil {
					serverTools[serverName] = make(map[string]config.Tool)
				}
				serverTools[serverName][name] = tool
			}

			// Get sorted list of server names
			var serverNames []string
			for server := range serverTools {
				serverNames = append(serverNames, server)
			}
			sort.Strings(serverNames)

			// Print each server's tools
			for _, serverName := range serverNames {
				fmt.Printf("%s:\n", serverName)

				// Get sorted tool names for this server
				var serverToolNames []string
				for name := range serverTools[serverName] {
					serverToolNames = append(serverToolNames, name)
				}
				sort.Strings(serverToolNames)

				// Print each tool's information
				for _, name := range serverToolNames {
					tool := serverTools[serverName][name]
					fmt.Printf("  %s:\n", name)
					fmt.Printf("    description: %s\n", tool.Description)
					fmt.Printf("    parameters:\n")

					// Get sorted parameter names
					var paramNames []string
					for paramName := range tool.Parameters.Properties {
						paramNames = append(paramNames, paramName)
					}
					sort.Strings(paramNames)

					// Print each parameter's information
					for _, paramName := range paramNames {
						prop := tool.Parameters.Properties[paramName]
						fmt.Printf("      %s:\n", paramName)
						fmt.Printf("        type: %s\n", prop.Type)
						if prop.Description != "" {
							fmt.Printf("        description: %s\n", prop.Description)
						}
						// Check if parameter is required
						for _, req := range tool.Parameters.Required {
							if req == paramName {
								fmt.Printf("        required: true\n")
								break
							}
						}
					}
				}
			}

			return nil
		},
	}
)
