package llm

import (
	"context"
	"fmt"

	"github.com/isaacphi/slop/internal/config"
	"github.com/isaacphi/slop/internal/domain"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/anthropic"
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

func (c *Client) Chat(ctx context.Context, content string, history []domain.Message) (string, error) {
	opts := []llms.CallOption{
		llms.WithTemperature(c.config.Temperature),
		llms.WithMaxTokens(c.config.MaxTokens),
		llms.WithTools(getTools(c.config.Tools)),
	}

	msgs := buildMessageHistory(history)
	msgs = append(msgs, llms.TextParts(llms.ChatMessageTypeHuman, content))

	resp, err := c.llm.GenerateContent(ctx, msgs, opts...)
	if err != nil {
		return "", fmt.Errorf("chat failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response choices returned")
	}

	return resp.Choices[0].Content, nil
}

func getTools(tools map[string]config.Tool) []llms.Tool {
	var result []llms.Tool

	for name, tool := range tools {
		// Convert our strongly typed Parameters to map[string]interface{}
		paramsMap := map[string]interface{}{
			"type":       tool.Parameters.Type,
			"properties": tool.Parameters.Properties,
			"required":   tool.Parameters.Required,
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

func (c *Client) ChatStream(ctx context.Context, content string, history []domain.Message, callback func(chunk []byte) error) error {
	wrappedCallback := func(ctx context.Context, chunk []byte) error {
		return callback(chunk)
	}

	opts := []llms.CallOption{
		llms.WithTemperature(c.config.Temperature),
		llms.WithMaxTokens(c.config.MaxTokens),
		llms.WithTools(getTools(c.config.Tools)),
		llms.WithStreamingFunc(wrappedCallback),
	}

	msgs := buildMessageHistory(history)
	msgs = append(msgs, llms.TextParts(llms.ChatMessageTypeHuman, content))

	_, err := c.llm.GenerateContent(ctx, msgs, opts...)
	if err != nil {
		return fmt.Errorf("streaming chat failed: %w", err)
	}

	return nil
}

func (c *Client) GetConfig() *config.Model {
	return c.config
}
