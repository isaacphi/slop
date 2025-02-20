package mcp

import (
	"context"
	"fmt"
	"os/exec"
	"runtime/debug"
	"strings"
	"sync"

	"github.com/isaacphi/slop/internal/config"
	mcp_golang "github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport/stdio"
	"github.com/pkg/errors"
)

// Client manages multiple MCP server connections
type Client struct {
	servers     map[string]config.MCPServer
	clients     map[string]*mcp_golang.Client
	commands    map[string]*exec.Cmd
	tools       map[string]Tool
	mu          sync.RWMutex
	initialized bool
}

// TODO: There should be a tool type in "agent"?
type Tool struct {
	Name        string
	FullName    string
	ServerName  string
	MCPClient   *mcp_golang.Client
	Description string
	Parameters  Parameters
}

type Parameters struct {
	Type       string              `mapstructure:"type" json:"type" jsonschema:"enum=object,default=object"`
	Properties map[string]Property `mapstructure:"properties" json:"properties" jsonschema:"description=Properties of the parameter object"`
	Required   []string            `mapstructure:"required" json:"required" jsonschema:"description=List of required property names"`
}

type Property struct {
	Type        string              `mapstructure:"type" json:"type" jsonschema:"description=JSON Schema type of the property"`
	Description string              `mapstructure:"description" json:"description" jsonschema:"description=Description of what the property does"`
	Enum        []string            `mapstructure:"enum,omitempty" json:"enum,omitempty" jsonschema:"description=Allowed values for this property"`
	Items       *Property           `mapstructure:"items,omitempty" json:"items,omitempty" jsonschema:"description=Schema for array items"`
	Properties  map[string]Property `mapstructure:"properties,omitempty" json:"properties,omitempty" jsonschema:"description=Nested properties for object types"`
	Required    []string            `mapstructure:"required,omitempty" json:"required,omitempty" jsonschema:"description=Required nested properties"`
	Default     interface{}         `mapstructure:"default,omitempty" json:"default,omitempty" jsonschema:"description=Default value for this property"`
}

// New creates a new MCP client manager
func New(servers map[string]config.MCPServer) *Client {
	return &Client{
		servers:  servers,
		clients:  make(map[string]*mcp_golang.Client),
		commands: make(map[string]*exec.Cmd),
		tools:    make(map[string]Tool),
	}
}

func (c *Client) Initialize(ctx context.Context) error {
	c.mu.Lock()
	if c.initialized {
		c.mu.Unlock()
		return errors.New("client already initialized")
	}
	c.mu.Unlock()

	var wg sync.WaitGroup
	errorsChan := make(chan error, len(c.servers)) // Buffered channel to collect errors

	// Start each server in parallel
	for name, server := range c.servers {
		name, server := name, server // Create local variables for closure
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := c.startServer(ctx, name, server); err != nil {
				errorsChan <- fmt.Errorf("server %s failed: %w", name, err)
			}
		}()
	}

	// Wait for all goroutines to finish
	wg.Wait()
	close(errorsChan)

	for err := range errorsChan {
		return fmt.Errorf("failed to initialize server: %v", err)
	}

	if err := c.buildToolRegistry(ctx); err != nil {
		c.Shutdown()
		return fmt.Errorf("failed to build tool registry: %w", err)
	}

	c.mu.Lock()
	c.initialized = true
	c.mu.Unlock()
	return nil
}

// startServer starts a single server and establishes its client connection
func (c *Client) startServer(ctx context.Context, name string, server config.MCPServer) error {
	parts := strings.Split(name, "__")
	if len(parts) != 1 {
		return fmt.Errorf("invalid server name format, can't contain '__', got '%s'", name)
	}

	cmd := exec.Command(server.Command, server.Args...)

	if server.Env != nil {
		for k, v := range server.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return errors.Wrap(err, "failed to get stdin pipe")
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return errors.Wrap(err, "failed to get stdout pipe")
	}

	if err := cmd.Start(); err != nil {
		return errors.Wrap(err, "failed to start server")
	}

	transport := stdio.NewStdioServerTransportWithIO(stdout, stdin)
	client := mcp_golang.NewClient(transport)

	// Initialize with client name and version
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return fmt.Errorf("No build info available")
	}
	if _, err := client.Initialize(ctx, "slop", info.Main.Version); err != nil {
		_ = cmd.Process.Kill()
		return errors.Wrap(err, "failed to initialize client")
	}

	c.mu.Lock()
	c.clients[name] = client
	c.commands[name] = cmd
	c.mu.Unlock()

	return nil
}

// buildToolRegistry creates a map of all available tools across all servers
func (c *Client) buildToolRegistry(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.tools = make(map[string]Tool)

	for serverName, client := range c.clients {
		response, err := client.ListTools(ctx, nil)
		if err != nil {
			return errors.Wrapf(err, "failed to list tools for server %s", serverName)
		}

		for _, mcpTool := range response.Tools {
			fullToolName := fmt.Sprintf("%s__%s", serverName, mcpTool.Name)

			description := ""
			if mcpTool.Description != nil {
				description = *mcpTool.Description
			}

			// Get schema from inputSchema which should be a map[string]interface{}
			var params Parameters
			if schema, ok := mcpTool.InputSchema.(map[string]interface{}); ok {
				params = parseSchema(schema)
			}

			c.tools[fullToolName] = Tool{
				Name:        mcpTool.Name,
				FullName:    fullToolName,
				ServerName:  serverName,
				MCPClient:   client,
				Description: description,
				Parameters:  params,
			}
		}
	}

	return nil
}

func parseSchema(schema map[string]interface{}) Parameters {
	params := Parameters{
		Properties: make(map[string]Property),
	}

	if t, ok := schema["type"].(string); ok {
		params.Type = t
	}

	if required, ok := schema["required"].([]interface{}); ok {
		params.Required = make([]string, 0, len(required))
		for _, r := range required {
			if str, ok := r.(string); ok {
				params.Required = append(params.Required, str)
			}
		}
	}

	if props, ok := schema["properties"].(map[string]interface{}); ok {
		for name, propInterface := range props {
			if propMap, ok := propInterface.(map[string]interface{}); ok {
				property := parseProperty(propMap)
				params.Properties[name] = property
			}
		}
	}

	return params
}

func parseProperty(propMap map[string]interface{}) Property {
	property := Property{}

	if t, ok := propMap["type"].(string); ok {
		property.Type = t
	}

	if desc, ok := propMap["description"].(string); ok {
		property.Description = desc
	}

	if enum, ok := propMap["enum"].([]interface{}); ok {
		property.Enum = make([]string, 0, len(enum))
		for _, e := range enum {
			if str, ok := e.(string); ok {
				property.Enum = append(property.Enum, str)
			}
		}
	}

	if def, ok := propMap["default"]; ok {
		property.Default = def
	}

	// Handle array items
	if items, ok := propMap["items"].(map[string]interface{}); ok {
		itemsProp := parseProperty(items)
		property.Items = &itemsProp
	}

	// Handle nested object properties
	if props, ok := propMap["properties"].(map[string]interface{}); ok {
		property.Properties = make(map[string]Property)
		for name, p := range props {
			if pMap, ok := p.(map[string]interface{}); ok {
				property.Properties[name] = parseProperty(pMap)
			}
		}
	}

	// Handle nested required fields
	if required, ok := propMap["required"].([]interface{}); ok {
		property.Required = make([]string, 0, len(required))
		for _, r := range required {
			if str, ok := r.(string); ok {
				property.Required = append(property.Required, str)
			}
		}
	}

	return property
}

// CallTool calls a tool using its fully qualified name (serverName__toolName)
func (c *Client) CallTool(ctx context.Context, name string, arguments interface{}) (*mcp_golang.ToolResponse, error) {
	parts := strings.Split(name, "__")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid tool name format, expected 'server__tool', got '%s'", name)
	}

	serverName, toolName := parts[0], parts[1]

	c.mu.RLock()
	client, exists := c.clients[serverName]
	c.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("server %s not found", serverName)
	}

	return client.CallTool(ctx, toolName, arguments)
}

// GetTools returns a map of all available tools
func (c *Client) GetTools() map[string]Tool {
	// TODO: edit this
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Create a copy of the tools map to prevent external modification
	tools := make(map[string]Tool, len(c.tools))
	for k, v := range c.tools {
		tools[k] = v
	}

	return tools
}

// Shutdown stops all servers and cleans up resources in parallel
func (c *Client) Shutdown() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.initialized {
		return
	}

	var wg sync.WaitGroup
	errs := make(chan error, len(c.commands))

	for name, cmd := range c.commands {
		if cmd != nil && cmd.Process != nil {
			wg.Add(1)
			go func(name string, cmd *exec.Cmd) {
				defer wg.Done()
				if err := cmd.Process.Kill(); err != nil {
					errs <- errors.Wrapf(err, "failed to kill server %s", name)
				}
			}(name, cmd)
		}
	}

	wg.Wait()
	close(errs)

	c.commands = make(map[string]*exec.Cmd)
	c.clients = make(map[string]*mcp_golang.Client)
	c.tools = make(map[string]Tool)
	c.initialized = false
}
