package llm

import (
	"context"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
)

type Client struct {
	model llms.Model
}

func NewClient(modelType string, apiKey string) (*Client, error) {
	var model llms.Model
	var err error

	switch modelType {
	case "openai":
		model, err = openai.New(openai.WithToken(apiKey))
	// Add more model types here
	default:
		model, err = openai.New(openai.WithToken(apiKey))
	}

	if err != nil {
		return nil, err
	}

	return &Client{
		model: model,
	}, nil
}

func (c *Client) SendMessage(ctx context.Context, prompt string) (string, error) {
	msg := llms.TextParts(llms.ChatMessageTypeHuman, prompt)
	completion, err := c.model.GenerateContent(ctx, []llms.MessageContent{msg})
	if err != nil {
		return "", err
	}

	if len(completion.Choices) > 0 {
		return completion.Choices[0].Content, nil
	}

	return "", nil
}

