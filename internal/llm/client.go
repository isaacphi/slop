package llm

import (
	"context"
	"fmt"
	"os"

	"github.com/isaacphi/slop/internal/config"
	"github.com/isaacphi/slop/internal/domain"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/anthropic"
	"github.com/tmc/langchaingo/llms/googleai"
	"github.com/tmc/langchaingo/llms/openai"
)

type Client struct {
	llm    llms.Model
	config *config.Model
}

func NewClient(modelCfg *config.Model) (*Client, error) {
	var llm llms.Model
	var err error

	switch modelCfg.Provider {
	case "openai":
		llm, err = openai.New(
			openai.WithModel(modelCfg.Name),
		)
	case "anthropic":
		llm, err = anthropic.New(
			anthropic.WithModel(modelCfg.Name),
		)
	case "googleai":
		genaiKey := os.Getenv("GEMINI_API_KEY")
		ctx := context.Background()
		llm, err = googleai.New(
			ctx,
			googleai.WithDefaultModel(modelCfg.Name),
			googleai.WithAPIKey(genaiKey),
		)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", modelCfg.Provider)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create %s client: %w", modelCfg.Provider, err)
	}

	return &Client{
		llm:    llm,
		config: modelCfg,
	}, nil
}

func buildMessageHistory(messages []domain.Message) []llms.MessageContent {
	var history []llms.MessageContent
	for _, msg := range messages {
		var role llms.ChatMessageType
		if msg.Role == domain.RoleAssistant {
			role = llms.ChatMessageTypeAI
		} else {
			role = llms.ChatMessageTypeHuman
		}
		history = append(history, llms.TextParts(role, msg.Content))
	}
	return history
}

func getTools(tools map[string]config.Tool) []llms.Tool {
	var result []llms.Tool
	for name, tool := range tools {
		// Convert properties to map[string]any
		properties := make(map[string]any)
		for pName, prop := range tool.Parameters.Properties {
			properties[pName] = map[string]any{
				"type":        prop.Type,
				"description": prop.Description,
			}
		}

		// Add rationale to properties
		properties["rationale"] = map[string]any{
			"type":        "string",
			"description": "The rationale for choosing this function call",
		}

		paramsMap := map[string]any{
			"type":       tool.Parameters.Type,
			"properties": properties,
			"required":   append([]string{"rationale"}, tool.Parameters.Required...),
		}

		langchainTool := llms.Tool{
			Type: tool.Type,
			Function: &llms.FunctionDefinition{
				Name:        name,
				Description: tool.Description,
				Parameters:  paramsMap,
			},
		}
		result = append(result, langchainTool)
	}
	return result
}

func (c *Client) GetConfig() *config.Model {
	return c.config
}

func (c *Client) Chat(ctx context.Context, content string, history []domain.Message, stream bool, callback func(chunk []byte) error) (string, error) {
	wrappedCallback := func(ctx context.Context, chunk []byte) error {
		return callback(chunk)
	}

	opts := []llms.CallOption{
		llms.WithTemperature(c.config.Temperature),
		llms.WithMaxTokens(c.config.MaxTokens),
	}

	// Convert tools to proper format
	tools := getTools(c.config.Tools)

	if len(tools) > 0 {
		opts = append(opts, llms.WithTools(tools))
	}

	if stream {
		opts = append(opts, llms.WithStreamingFunc(wrappedCallback))
	}

	msgs := buildMessageHistory(history)
	msgs = append(msgs, llms.TextParts(llms.ChatMessageTypeHuman, content))

	resp, err := c.llm.GenerateContent(ctx, msgs, opts...)
	if err != nil {
		return "", fmt.Errorf("streaming chat failed: %w", err)
	}

	// Handle function calls in the response
	if len(resp.Choices) > 0 && resp.Choices[0].FuncCall != nil {
		fmt.Printf("Function call: %+v\n", resp.Choices[0].FuncCall)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response choices returned")
	}

	// Log the full response details
	fmt.Printf("Response object: %+v\n", resp)
	for i, choice := range resp.Choices {
		fmt.Printf("Choice %d:\n", i)
		fmt.Printf("  Content: %s\n", choice.Content)
		fmt.Printf("  StopReason: %s\n", choice.StopReason)
		fmt.Printf("  GenerationInfo: %+v\n", choice.GenerationInfo)
		if len(choice.ToolCalls) > 0 {
			fmt.Printf("  ToolCalls:\n")
			for _, tc := range choice.ToolCalls {
				fmt.Printf("    ID: %s\n", tc.ID)
				fmt.Printf("    Function: %+v\n", tc.FunctionCall)
			}
		}
	}
	return resp.Choices[0].Content, nil
}
