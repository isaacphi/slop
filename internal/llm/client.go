package llm

import (
	"context"
	"fmt"

	"github.com/isaacphi/wheel/internal/config"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/anthropic"
	"github.com/tmc/langchaingo/llms/openai"
)

type Client struct {
	llm    llms.Model
	config *config.Model
}

func NewClient(cfg *config.ConfigSchema) (*Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	modelCfg, ok := cfg.Models[cfg.ActiveModel]
	if !ok {
		return nil, fmt.Errorf("model %s not found in configuration", cfg.ActiveModel)
	}

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
		config: &modelCfg,
	}, nil
}

func (c *Client) Chat(ctx context.Context, content string) (string, error) {
	opts := []llms.CallOption{
		llms.WithTemperature(c.config.Temperature),
		llms.WithMaxLength(c.config.MaxLength),
	}

	msgs := []llms.MessageContent{
		llms.TextParts("human", content),
	}

	resp, err := c.llm.GenerateContent(ctx, msgs, opts...)
	if err != nil {
		return "", fmt.Errorf("chat failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response choices returned")
	}

	return resp.Choices[0].Content, nil
}

func (c *Client) ChatStream(ctx context.Context, content string, callback func(chunk []byte) error) error {
	wrappedCallback := func(ctx context.Context, chunk []byte) error {
		return callback(chunk)
	}

	opts := []llms.CallOption{
		llms.WithTemperature(c.config.Temperature),
		llms.WithMaxLength(c.config.MaxLength),
		llms.WithStreamingFunc(wrappedCallback),
	}

	msgs := []llms.MessageContent{
		llms.TextParts("human", content),
	}

	_, err := c.llm.GenerateContent(ctx, msgs, opts...)
	if err != nil {
		return fmt.Errorf("streaming chat failed: %w", err)
	}

	return nil
}

func (c *Client) GetConfig() *config.Model {
	return c.config
}

