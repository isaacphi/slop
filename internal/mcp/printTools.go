package mcp

import (
	"fmt"
	"sort"
)

func (c *Client) PrintTools() {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Already grouped by server in the new structure
	for serverName, tools := range c.tools {
		fmt.Printf("%s:\n", serverName)

		// Sort tool names for consistent output
		toolNames := make([]string, 0, len(tools))
		for toolName := range tools {
			toolNames = append(toolNames, toolName)
		}
		sort.Strings(toolNames)

		for _, toolName := range toolNames {
			tool := tools[toolName]
			fmt.Printf("  %s:\n", toolName)
			fmt.Printf("    description: %s\n", tool.Description)
			fmt.Printf("    parameters:\n")

			if tool.Parameters.Type != "" {
				fmt.Printf("      type: %s\n", tool.Parameters.Type)
			}

			if len(tool.Parameters.Required) > 0 {
				fmt.Printf("      required:\n")
				for _, req := range tool.Parameters.Required {
					fmt.Printf("        - %s\n", req)
				}
			}

			if len(tool.Parameters.Properties) > 0 {
				fmt.Printf("      properties:\n")

				// Sort property names for consistent output
				propNames := make([]string, 0, len(tool.Parameters.Properties))
				for propName := range tool.Parameters.Properties {
					propNames = append(propNames, propName)
				}
				sort.Strings(propNames)

				for _, propName := range propNames {
					prop := tool.Parameters.Properties[propName]
					fmt.Printf("        %s:\n", propName)
					if prop.Type != "" {
						fmt.Printf("          type: %s\n", prop.Type)
					}
					if prop.Description != "" {
						fmt.Printf("          description: %s\n", prop.Description)
					}
					if len(prop.Enum) > 0 {
						fmt.Printf("          enum:\n")
						sort.Strings(prop.Enum) // Sort enums for consistent output
						for _, enum := range prop.Enum {
							fmt.Printf("            - %s\n", enum)
						}
					}
					if prop.Default != nil {
						fmt.Printf("          default: %v\n", prop.Default)
					}
					if prop.Items != nil {
						fmt.Printf("          items:\n")
						fmt.Printf("            type: %s\n", prop.Items.Type)
						if prop.Items.Description != "" {
							fmt.Printf("            description: %s\n", prop.Items.Description)
						}
					}
				}
			}
			fmt.Println()
		}
	}
}
