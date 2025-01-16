package llm

import (
	"context"
	"fmt"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
)

type Client struct {
	llm llms.Model
}

type Options struct {
	Model       string
	Temperature float64
	MaxTokens   int
}

func NewClient(opts *Options) (*Client, error) {
	if opts == nil {
		opts = &Options{
			Model:       "gpt-3.5-turbo",
			Temperature: 0.7,
			MaxTokens:   2000,
		}
	}

	llm, err := openai.New(
		openai.WithModel(opts.Model),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenAI client: %w", err)
	}

	return &Client{
		llm: llm,
	}, nil
}

func (c *Client) Chat(ctx context.Context, content string) (string, error) {
	msgs := []llms.MessageContent{
		llms.TextParts("human", content),
	}

	resp, err := c.llm.GenerateContent(ctx, msgs)
	if err != nil {
		return "", fmt.Errorf("chat failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response choices returned")
	}

	return resp.Choices[0].Content, nil
}

func (c *Client) ChatStream(ctx context.Context, content string, callback func(chunk []byte) error) error {
	msgs := []llms.MessageContent{
		llms.TextParts("human", content),
	}

	_, err := c.llm.GenerateContent(ctx, msgs,
		llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			return callback(chunk)
		}),
	)
	if err != nil {
		return fmt.Errorf("streaming chat failed: %w", err)
	}

	return nil
}
