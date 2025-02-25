package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/isaacphi/slop/internal/config"
	"github.com/isaacphi/slop/internal/domain"
	"github.com/isaacphi/slop/internal/llm"
)

type toolWithApproval struct {
	domain.Tool
	RequireApproval bool
}

func flattenTools(tools map[string]map[string]toolWithApproval) map[string]domain.Tool {
	flat := make(map[string]domain.Tool)
	for server, serverTools := range tools {
		for name, tool := range serverTools {
			flat[fmt.Sprintf("%s__%s", server, name)] = tool.Tool
		}
	}
	return flat
}

func filterAndModifyTools(allTools map[string]map[string]domain.Tool, modelToolsets []string, toolsets map[string]config.Toolset) (map[string]map[string]toolWithApproval, error) {
	result := make(map[string]map[string]toolWithApproval)

	for _, toolsetName := range modelToolsets {
		toolset := toolsets[toolsetName]

		for serverName, serverConfig := range toolset.Servers {
			serverTools, exists := allTools[serverName]
			if !exists {
				return nil, fmt.Errorf("server %q not found", serverName)
			}

			if _, exists := result[serverName]; !exists {
				result[serverName] = make(map[string]toolWithApproval)
			}

			// If AllowedTools is empty, include all server tools with server-level approval
			if len(serverConfig.AllowedTools) == 0 {
				for toolName, tool := range serverTools {
					result[serverName][toolName] = toolWithApproval{
						Tool:            tool,
						RequireApproval: serverConfig.RequireApproval,
					}
				}
				continue
			}

			// Process specific allowed tools
			for toolName, toolConfig := range serverConfig.AllowedTools {
				tool, exists := serverTools[toolName]
				if !exists {
					return nil, fmt.Errorf("tool %q not found in server %q", toolName, serverName)
				}

				if len(toolConfig.PresetParameters) > 0 {
					tool = modifyToolWithPresets(tool, toolConfig.PresetParameters)
				}

				result[serverName][toolName] = toolWithApproval{
					Tool:            tool,
					RequireApproval: toolConfig.RequireApproval,
				}
			}
		}
	}

	return result, nil
}

func modifyToolWithPresets(original domain.Tool, presets map[string]string) domain.Tool {
	modified := original

	// Create new properties map
	newProps := make(map[string]domain.Property)
	newRequired := make([]string, 0)

	// Copy non-preset parameters
	for name, prop := range original.Parameters.Properties {
		if _, isPreset := presets[name]; !isPreset {
			newProps[name] = prop
			// Only include in required if it's not preset
			for _, req := range original.Parameters.Required {
				if req == name {
					newRequired = append(newRequired, name)
				}
			}
		}
	}

	modified.Parameters = domain.Parameters{
		Type:       original.Parameters.Type,
		Properties: newProps,
		Required:   newRequired,
	}

	return modified
}

// ExecuteTools executes a set of tool calls and returns the formatted results
func (a *Agent) ExecuteTools(ctx context.Context, toolCalls []llm.ToolCall) (string, error) {
	// Create channels for collecting results
	type toolResult struct {
		call   llm.ToolCall
		result string
		err    error
	}

	resultChan := make(chan toolResult, len(toolCalls))

	// Execute tools concurrently
	for _, call := range toolCalls {
		go func(tc llm.ToolCall) {
			select {
			case <-ctx.Done():
				resultChan <- toolResult{
					call:   tc,
					result: "",
					err:    ctx.Err(),
				}
				return
			default:
				result, err := a.executeFunction(ctx, tc, a.tools)
				resultChan <- toolResult{
					call:   tc,
					result: result,
					err:    err,
				}
			}
		}(call)
	}

	// Collect all results
	var combinedResults strings.Builder
	combinedResults.WriteString("Tool call results:\n\n")

	for i := 0; i < len(toolCalls); i++ {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case res := <-resultChan:
			// Format the tool call header
			fmt.Fprintf(&combinedResults, "Name: %s\n", res.call.Name)
			fmt.Fprintf(&combinedResults, "ID: %s\n", res.call.ID)
			fmt.Fprintf(&combinedResults, "Arguments: %s\n", string(res.call.Arguments))
			fmt.Fprint(&combinedResults, "Result:\n")

			// Add result or error
			if res.err != nil {
				fmt.Fprintf(&combinedResults, "Error: %v\n", res.err)
			} else {
				fmt.Fprintf(&combinedResults, "%s\n", res.result)
			}

			// Add separator between results unless it's the last one
			if i < len(toolCalls)-1 {
				combinedResults.WriteString("\n")
			}
		}
	}

	return combinedResults.String(), nil
}

// validateArguments checks if the provided arguments match the tool's schema
func validateArguments(args json.RawMessage, tool toolWithApproval) error {
	var parsedArgs map[string]interface{}
	if err := json.Unmarshal(args, &parsedArgs); err != nil {
		return fmt.Errorf("invalid argument format: %w", err)
	}

	// Check required parameters
	for _, required := range tool.Parameters.Required {
		if _, exists := parsedArgs[required]; !exists {
			return fmt.Errorf("missing required parameter: %s", required)
		}
	}

	// Validate each provided argument
	for name, value := range parsedArgs {
		prop, exists := tool.Parameters.Properties[name]
		if !exists {
			return fmt.Errorf("unknown parameter: %s", name)
		}

		// Type validation
		switch prop.Type {
		case "string":
			if _, ok := value.(string); !ok {
				return fmt.Errorf("parameter %s must be a string", name)
			}
		case "number":
			if _, ok := value.(float64); !ok {
				return fmt.Errorf("parameter %s must be a number", name)
			}
		case "boolean":
			if _, ok := value.(bool); !ok {
				return fmt.Errorf("parameter %s must be a boolean", name)
			}
		case "array":
			if _, ok := value.([]interface{}); !ok {
				return fmt.Errorf("parameter %s must be an array", name)
			}
		}

		// Enum validation
		if len(prop.Enum) > 0 {
			if strVal, ok := value.(string); ok {
				valid := false
				for _, enum := range prop.Enum {
					if strVal == enum {
						valid = true
						break
					}
				}
				if !valid {
					return fmt.Errorf("parameter %s must be one of: %v", name, prop.Enum)
				}
			}
		}
	}

	return nil
}

func (a *Agent) executeFunction(ctx context.Context, toolCall llm.ToolCall, tools map[string]map[string]toolWithApproval) (string, error) {
	// Find the tool
	for serverName, serverTools := range tools {
		for toolName, tool := range serverTools {
			if fmt.Sprintf("%s__%s", serverName, toolName) == toolCall.Name {
				// Parse provided arguments
				var providedArgs map[string]interface{}
				if err := json.Unmarshal(toolCall.Arguments, &providedArgs); err != nil {
					return "", fmt.Errorf("failed to parse arguments: %w", err)
				}

				// Check if any parameters were preset
				originalTools := a.mcpClient.GetTools()
				originalTool := originalTools[serverName][toolName]

				// Find preset parameters by comparing schemas
				presetParams := make(map[string]string)
				for paramName := range originalTool.Parameters.Properties {
					if _, exists := tool.Parameters.Properties[paramName]; !exists {
						// Parameter was preset - find its value
						for _, toolsetName := range a.preset.Toolsets {
							if toolset, ok := a.toolsets[toolsetName]; ok {
								if serverConfig, ok := toolset.Servers[serverName]; ok {
									if toolConfig, ok := serverConfig.AllowedTools[toolName]; ok {
										if value, ok := toolConfig.PresetParameters[paramName]; ok {
											presetParams[paramName] = value
										}
									}
								}
							}
						}
					}
				}

				// Merge preset parameters with provided arguments
				mergedArgs := make(map[string]interface{})
				for k, v := range presetParams {
					mergedArgs[k] = v
				}
				for k, v := range providedArgs {
					mergedArgs[k] = v
				}

				// Validate against tool schema
				if err := validateArguments(toolCall.Arguments, tool); err != nil {
					return "", fmt.Errorf("argument validation failed: %w", err)
				}

				// Execute the function
				result, err := a.mcpClient.CallTool(ctx, serverName, toolName, mergedArgs)
				if err != nil {
					return "", fmt.Errorf("function execution failed: %w", err)
				}

				resultBytes, err := json.Marshal(result)
				if err != nil {
					return "", fmt.Errorf("failed to format result: %w", err)
				}

				return string(resultBytes), nil
			}
		}
	}

	return "", fmt.Errorf("tool %s not found", toolCall.Name)
}
