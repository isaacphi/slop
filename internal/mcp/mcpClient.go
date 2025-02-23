package mcp

import (
	"context"
	"fmt"
	"os/exec"
	"runtime/debug"
	"strings"
	"sync"

	"github.com/isaacphi/slop/internal/config"
	"github.com/isaacphi/slop/internal/domain"
	mcp_golang "github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport/stdio"
	"github.com/pkg/errors"
)

// Client manages multiple MCP server connections
type Client struct {
	servers     map[string]config.MCPServer
	clients     map[string]*mcp_golang.Client
	commands    map[string]*exec.Cmd
	tools       map[string]map[string]domain.Tool
	mu          sync.RWMutex
	initialized bool
}

// New creates a new MCP client manager
func New(servers map[string]config.MCPServer) *Client {
	return &Client{
		servers:  servers,
		clients:  make(map[string]*mcp_golang.Client),
		commands: make(map[string]*exec.Cmd),
		tools:    make(map[string]map[string]domain.Tool),
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

func (c *Client) buildToolRegistry(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.tools = make(map[string]map[string]domain.Tool)

	for serverName, client := range c.clients {
		response, err := client.ListTools(ctx, nil)
		if err != nil {
			return errors.Wrapf(err, "failed to list tools for server %s", serverName)
		}

		c.tools[serverName] = make(map[string]domain.Tool)

		for _, mcpTool := range response.Tools {
			description := ""
			if mcpTool.Description != nil {
				description = *mcpTool.Description
			}

			var params domain.Parameters
			if schema, ok := mcpTool.InputSchema.(map[string]interface{}); ok {
				params = parseSchema(schema)
			}

			c.tools[serverName][mcpTool.Name] = domain.Tool{
				Name:        mcpTool.Name,
				Description: description,
				Parameters:  params,
			}
		}
	}

	return nil
}

func parseSchema(schema map[string]interface{}) domain.Parameters {
	params := domain.Parameters{
		Properties: make(map[string]domain.Property),
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

func parseProperty(propMap map[string]interface{}) domain.Property {
	property := domain.Property{}

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
		property.Properties = make(map[string]domain.Property)
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

// CallTool calls a tool on a specific server
func (c *Client) CallTool(ctx context.Context, serverName string, toolName string, arguments interface{}) (*mcp_golang.ToolResponse, error) {
	c.mu.RLock()
	client, exists := c.clients[serverName]
	c.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("server %s not found", serverName)
	}

	return client.CallTool(ctx, toolName, arguments)
}

func (c *Client) GetTools() map[string]map[string]domain.Tool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]map[string]domain.Tool)
	for server, tools := range c.tools {
		result[server] = make(map[string]domain.Tool)
		for name, tool := range tools {
			result[server][name] = tool
		}
	}

	return result
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
	c.tools = make(map[string]map[string]domain.Tool)
	c.initialized = false
}
