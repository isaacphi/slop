package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/isaacphi/slop/internal/config"
	"github.com/isaacphi/slop/internal/domain"
	"github.com/isaacphi/slop/internal/mcp"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/anthropic"
	"github.com/tmc/langchaingo/llms/googleai"
	"github.com/tmc/langchaingo/llms/openai"
)

type MessageResponse struct {
	TextResponse string
	ToolCalls    []ToolCall
}

type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type StreamChunk interface {
	Raw() []byte
}

type FunctionCallChunk struct {
	Name          string `json:"name"`
	ArgumentsJson string `json:"arguments"`
}

type StreamHandler interface {
	HandleTextChunk(chunk []byte) error
	HandleMessageDone() error
	HandleFunctionCallStart(id, name string) error
	HandleFunctionCallChunk(chunk FunctionCallChunk) error
	Reset()
}

func createLLMClient(modelCfg config.ModelPreset) (llms.Model, error) {
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

	return llm, nil
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

func getTools(tools map[string]mcp.Tool) []llms.Tool {
	var result []llms.Tool
	for name, tool := range tools {
		paramsMap := convertParameters(tool.Parameters)

		langchainTool := llms.Tool{
			Type: "function",
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

func convertParameters(params mcp.Parameters) map[string]any {
	properties := make(map[string]any)

	for pName, prop := range params.Properties {
		properties[pName] = convertProperty(prop)
	}

	return map[string]any{
		"type":       params.Type,
		"properties": properties,
		"required":   params.Required,
	}
}

func convertProperty(prop mcp.Property) map[string]any {
	result := map[string]any{
		"type":        prop.Type,
		"description": prop.Description,
	}

	if len(prop.Enum) > 0 {
		result["enum"] = prop.Enum
	}

	if prop.Default != nil {
		result["default"] = prop.Default
	}

	if prop.Type == "array" && prop.Items != nil {
		result["items"] = convertProperty(*prop.Items)
	}

	if prop.Type == "object" && len(prop.Properties) > 0 {
		nestedProps := make(map[string]any)
		for name, p := range prop.Properties {
			nestedProps[name] = convertProperty(p)
		}
		result["properties"] = nestedProps

		if len(prop.Required) > 0 {
			result["required"] = prop.Required
		}
	}

	return result
}

// GenerateContent generates content using the specified model configuration
func GenerateContent(
	ctx context.Context,
	modelCfg config.ModelPreset,
	content string,
	history []domain.Message,
	tools map[string]mcp.Tool,
	streamHandler StreamHandler,
) (MessageResponse, error) {
	llmClient, err := createLLMClient(modelCfg)
	if err != nil {
		return MessageResponse{}, fmt.Errorf("failed to create LLM client: %w", err)
	}

	var streamCallback func(ctx context.Context, chunk []byte) error
	if streamHandler != nil {
		streamCallback = func(ctx context.Context, chunk []byte) error {
			return streamHandler.HandleTextChunk(chunk)
		}
	}

	opts := []llms.CallOption{
		llms.WithTemperature(modelCfg.Temperature),
		llms.WithMaxTokens(modelCfg.MaxTokens),
	}

	langchainTools := getTools(tools)
	if len(langchainTools) > 0 {
		opts = append(opts, llms.WithTools(langchainTools))
	}

	if streamHandler != nil {
		opts = append(opts, llms.WithStreamingFunc(streamCallback))
	}

	msgs := buildMessageHistory(history)
	msgs = append(msgs, llms.TextParts(llms.ChatMessageTypeHuman, content))

	resp, err := llmClient.GenerateContent(ctx, msgs, opts...)
	if err != nil {
		return MessageResponse{}, fmt.Errorf("streaming message failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return MessageResponse{}, fmt.Errorf("no response choices returned")
	}

	toolCalls := make([]ToolCall, 0)
	for _, choice := range resp.Choices {
		if len(choice.ToolCalls) > 0 {
			for _, tc := range choice.ToolCalls {
				toolCalls = append(toolCalls, ToolCall{
					ID:        tc.ID,
					Name:      tc.FunctionCall.Name,
					Arguments: json.RawMessage(tc.FunctionCall.Arguments),
				})
			}
		}
	}

	return MessageResponse{
		TextResponse: resp.Choices[0].Content,
		ToolCalls:    toolCalls,
	}, nil
}
