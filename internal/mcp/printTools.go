package mcp

import "fmt"

func (c *Client) PrintTools() {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Group tools by server
	serverTools := make(map[string][]Tool)
	for _, tool := range c.tools {
		serverTools[tool.ServerName] = append(serverTools[tool.ServerName], tool)
	}

	// Print in YAML format
	for serverName, tools := range serverTools {
		fmt.Printf("%s:\n", serverName)
		for _, tool := range tools {
			fmt.Printf("  %s:\n", tool.Name)
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
				for propName, prop := range tool.Parameters.Properties {
					fmt.Printf("        %s:\n", propName)
					if prop.Type != "" {
						fmt.Printf("          type: %s\n", prop.Type)
					}
					if prop.Description != "" {
						fmt.Printf("          description: %s\n", prop.Description)
					}
					if len(prop.Enum) > 0 {
						fmt.Printf("          enum:\n")
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
