package llm

import (
	"context"
	"io"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
)

type StreamHandler func(chunk string)

type Client struct {
	model llms.Model
}

func NewClient(modelType string, apiKey string) (*Client, error) {
	var model llms.Model
	var err error

	switch modelType {
	case "openai":
		model, err = openai.New(
			openai.WithToken(apiKey),
			openai.WithModel("gpt-4-turbo-preview"),
		)
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
	
	completion, err := c.model.GenerateContent(
		ctx, 
		[]llms.MessageContent{msg},
		llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			// For now we're not using streaming, but it's ready to be implemented
			return nil
		}),
	)
	
	if err != nil {
		return "", err
	}

	if len(completion.Choices) > 0 {
		return completion.Choices[0].Content, nil
	}
	
	return "", nil
}

func (c *Client) SendMessageStream(ctx context.Context, prompt string, handler StreamHandler) error {
	msg := llms.TextParts(llms.ChatMessageTypeHuman, prompt)
	
	_, err := c.model.GenerateContent(
		ctx, 
		[]llms.MessageContent{msg},
		llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			handler(string(chunk))
			return nil
		}),
	)
	
	return err
}