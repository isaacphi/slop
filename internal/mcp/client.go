package mcp

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"

	"github.com/isaacphi/slop/internal/config"
	mcp_golang "github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport/stdio"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

// Client manages multiple MCP server connections
type Client struct {
	servers     map[string]config.MCPServer
	clients     map[string]*mcp_golang.Client
	commands    map[string]*exec.Cmd
	tools       map[string]config.Tool
	mu          sync.RWMutex
	initialized bool
}

// New creates a new MCP client manager
func New(servers map[string]config.MCPServer) *Client {
	return &Client{
		servers:  servers,
		clients:  make(map[string]*mcp_golang.Client),
		commands: make(map[string]*exec.Cmd),
		tools:    make(map[string]config.Tool),
	}
}

// Initialize starts all configured servers and establishes connections in parallel
func (c *Client) Initialize(ctx context.Context) error {
	c.mu.Lock()
	if c.initialized {
		c.mu.Unlock()
		return errors.New("client already initialized")
	}
	c.mu.Unlock()

	g, gctx := errgroup.WithContext(ctx)

	// Start each server in parallel
	for name, server := range c.servers {
		name, server := name, server // Create local variables for closure
		g.Go(func() error {
			return c.startServer(gctx, name, server)
		})
	}

	if err := g.Wait(); err != nil {
		c.Shutdown()
		return errors.Wrap(err, "failed to initialize servers")
	}

	if err := c.buildToolRegistry(ctx); err != nil {
		c.Shutdown()
		return errors.Wrap(err, "failed to build tool registry")
	}

	c.mu.Lock()
	c.initialized = true
	c.mu.Unlock()

	return nil
}

// startServer starts a single server and establishes its client connection
func (c *Client) startServer(ctx context.Context, name string, server config.MCPServer) error {
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
	if _, err := client.Initialize(ctx, fmt.Sprintf("slop-%s", name), "1.0.0"); err != nil {
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

	c.tools = make(map[string]config.Tool)

	for serverName, client := range c.clients {
		response, err := client.ListTools(ctx, nil)
		if err != nil {
			return errors.Wrapf(err, "failed to list tools for server %s", serverName)
		}

		for _, mcpTool := range response.Tools {
			toolName := fmt.Sprintf("%s.%s", serverName, mcpTool.Name)

			description := ""
			if mcpTool.Description != nil {
				description = *mcpTool.Description
			}

			// Get schema from inputSchema which should be a map[string]interface{}
			var params config.Parameters
			if schema, ok := mcpTool.InputSchema.(map[string]interface{}); ok {
				params = parseSchema(schema)
			}

			c.tools[toolName] = config.Tool{
				Name:        toolName,
				Description: description,
				Parameters:  params,
			}
		}
	}

	return nil
}

// parseSchema converts a JSON schema map into Parameters struct
func parseSchema(schema map[string]interface{}) config.Parameters {
	params := config.Parameters{
		Properties: make(map[string]config.Property),
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
				property := config.Property{}

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

				params.Properties[name] = property
			}
		}
	}

	return params
}

// CallTool calls a tool using its fully qualified name (serverName.toolName)
func (c *Client) CallTool(ctx context.Context, name string, arguments interface{}) (*mcp_golang.ToolResponse, error) {
	parts := strings.SplitN(name, ".", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid tool name format, expected 'server.tool', got '%s'", name)
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
func (c *Client) GetTools() map[string]config.Tool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Create a copy of the tools map to prevent external modification
	tools := make(map[string]config.Tool, len(c.tools))
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
	c.tools = make(map[string]config.Tool)
	c.initialized = false
}
