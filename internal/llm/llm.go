package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/isaacphi/slop/internal/config"
	"github.com/isaacphi/slop/internal/domain"
	"github.com/isaacphi/slop/internal/events"
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
	HandleMessageDone()
	HandleFunctionCallStart(id, name string) error
	HandleFunctionCallChunk(chunk FunctionCallChunk) error
}

func createLLMClient(preset config.Preset) (llms.Model, error) {
	var llm llms.Model
	var err error

	switch preset.Provider {
	case "openai":
		llm, err = openai.New(
			openai.WithModel(preset.Name),
		)
	case "anthropic":
		llm, err = anthropic.New(
			anthropic.WithModel(preset.Name),
		)
	case "googleai":
		genaiKey := os.Getenv("GEMINI_API_KEY")
		ctx := context.Background()
		llm, err = googleai.New(
			ctx,
			googleai.WithDefaultModel(preset.Name),
			googleai.WithAPIKey(genaiKey),
		)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", preset.Provider)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create %s client: %w", preset.Provider, err)
	}

	return llm, nil
}

func buildMessageHistory(systemMessage *domain.Message, messages []domain.Message) []llms.MessageContent {
	var history []llms.MessageContent
	if systemMessage != nil {
		history = append(history, llms.TextParts(llms.ChatMessageTypeSystem, systemMessage.Content))
	}

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

func getTools(tools map[string]domain.Tool) []llms.Tool {
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

func convertParameters(params domain.Parameters) map[string]any {
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

func convertProperty(prop domain.Property) map[string]any {
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

type GenerateContentOptions struct {
	Preset        config.Preset
	Content       string
	SystemMessage *domain.Message
	History       []domain.Message
	Tools         map[string]domain.Tool
	StreamHandler StreamHandler
}

// GenerateContentStream returns a stream of events from the LLM
func GenerateContentStream(
	ctx context.Context,
	opts GenerateContentOptions,
) LLMStream {
	eventsChan := make(chan events.Event)
	done := make(chan struct{})

	go func() {
		defer close(done)
		defer close(eventsChan)

		// Check for context cancellation
		select {
		case <-ctx.Done():
			eventsChan <- &events.ErrorEvent{Error: ctx.Err()}
			return
		default:
		}

		llmClient, err := createLLMClient(opts.Preset)
		if err != nil {
			eventsChan <- &events.ErrorEvent{Error: fmt.Errorf("failed to create LLM client: %w", err)}
			return
		}

		// Map to track parsers for different function calls
		var toolCallParsers = make(map[string]*ToolCallArgumentParser)
		var functionId *string
		var functionName string

		// In the streaming callback
		streamCallback := func(ctx context.Context, chunk []byte) error {
			// Try to parse as function call first
			var fcall []struct {
				Function FunctionCallChunk `json:"function"`
				Id       *string           `json:"id,omitempty"`
			}
			if err := json.Unmarshal(chunk, &fcall); err == nil && len(fcall) > 0 {
				// This is a function call chunk
				if fcall[0].Function.Name != functionName {
					functionName = fcall[0].Function.Name
					eventsChan <- &ToolCallStartEvent{FunctionName: functionName}
				}

				// OpenAI only returns the function call ID once so we perist it
				if fcall[0].Id != nil {
					functionId = fcall[0].Id
				}
				if functionId == nil {
					return nil
				}

				// Get or create a parser for this function call
				parser, exists := toolCallParsers[*functionId]
				if !exists {
					parser = NewToolCallArgumentParser()
					toolCallParsers[*functionId] = parser
				}

				// Add the chunk to the parser and get updates
				updates := parser.AddChunk(fcall[0].Function.ArgumentsJson)

				// Emit events for each update
				for _, update := range updates {
					switch e := update.(type) {
					case ToolNewArgumentEvent:
						e.ToolCallID = *functionId
						e.Name = functionName
						eventsChan <- &e
					case ToolArgumentChunkEvent:
						if e.Chunk != "" {
							e.ToolCallID = *functionId
							e.Name = functionName
							eventsChan <- &e
						}
					}
				}
				return nil
			}

			// Regular text chunk
			eventsChan <- &TextEvent{Content: string(chunk)}
			return nil
		}

		callOptions := []llms.CallOption{
			llms.WithTemperature(opts.Preset.Temperature),
			llms.WithMaxTokens(opts.Preset.MaxTokens),
			llms.WithStreamingFunc(streamCallback),
		}

		langchainTools := getTools(opts.Tools)
		if len(langchainTools) > 0 {
			callOptions = append(callOptions, llms.WithTools(langchainTools))
		}

		if opts.SystemMessage != nil && opts.SystemMessage.Role != domain.RoleSystem {
			eventsChan <- &events.ErrorEvent{Error: fmt.Errorf("system message is of type %v", opts.SystemMessage.Role)}
			return
		}

		msgs := buildMessageHistory(opts.SystemMessage, opts.History)
		msgs = append(msgs, llms.TextParts(llms.ChatMessageTypeHuman, opts.Content))

		resp, err := llmClient.GenerateContent(ctx, msgs, callOptions...)
		if err != nil {
			eventsChan <- &events.ErrorEvent{Error: fmt.Errorf("streaming message failed: %w", err)}
			return
		}

		// Send complete response with tool calls
		if len(resp.Choices) > 0 {
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

			// TODO: can there be text content in other choices? Might need to combine them
			eventsChan <- &MessageCompleteEvent{
				Content:   resp.Choices[0].Content,
				ToolCalls: toolCalls,
			}
		}
	}()

	return LLMStream{Events: eventsChan, Done: done}
}

// TODO: Remove
// GenerateContent generates content using the specified model configuration
func GenerateContent(
	ctx context.Context,
	opts GenerateContentOptions,
) (MessageResponse, error) {
	llmClient, err := createLLMClient(opts.Preset)
	if err != nil {
		return MessageResponse{}, fmt.Errorf("failed to create LLM client: %w", err)
	}

	var streamCallback func(ctx context.Context, chunk []byte) error

	if opts.StreamHandler != nil {
		// Closure to track streaming state
		var currentFunctionId string

		streamCallback = func(ctx context.Context, chunk []byte) error {
			// Try to parse as function call first
			// When a function call is about to start, a chunk with the following format is sent:
			// [{"function":{"arguments":"","name":"filesystem__read_file"},"id":"toolu_01TA4sQsjA1XhWDBBof9THGJ","type":"function"}]
			// Subsequent chunks take the form below, with "arguments" containing incremental chunks of the arguments json
			// [{"function":{"arguments":"ml\"}","name":"filesystem__read_file"},"id":"toolu_01TA4sQsjA1XhWDBBof9THGJ","type":"function"}]
			var fcall []struct {
				Function FunctionCallChunk `json:"function"`
				Id       *string           `json:"id,omitempty"`
			}
			if err := json.Unmarshal(chunk, &fcall); err == nil && len(fcall) > 0 {
				// This is a function call chunk
				functionName := fcall[0].Function.Name
				functionId := fcall[0].Id
				if functionId != nil && currentFunctionId != *functionId {
					if err := opts.StreamHandler.HandleFunctionCallStart(*functionId, functionName); err != nil {
						return err
					}
					currentFunctionId = *functionId
				}
				return opts.StreamHandler.HandleFunctionCallChunk(fcall[0].Function)
			}
			// Regular text chunk
			return opts.StreamHandler.HandleTextChunk(chunk)
		}
	}

	callOptions := []llms.CallOption{
		llms.WithTemperature(opts.Preset.Temperature),
		llms.WithMaxTokens(opts.Preset.MaxTokens),
	}

	langchainTools := getTools(opts.Tools)
	if len(langchainTools) > 0 {
		callOptions = append(callOptions, llms.WithTools(langchainTools))
	}

	if opts.StreamHandler != nil {
		callOptions = append(callOptions, llms.WithStreamingFunc(streamCallback))
	}

	if opts.SystemMessage != nil && opts.SystemMessage.Role != domain.RoleSystem {
		return MessageResponse{}, fmt.Errorf("system message is of type %v", opts.SystemMessage.Role)
	}
	msgs := buildMessageHistory(opts.SystemMessage, opts.History)
	msgs = append(msgs, llms.TextParts(llms.ChatMessageTypeHuman, opts.Content))

	resp, err := llmClient.GenerateContent(ctx, msgs, callOptions...)
	if err != nil {
		return MessageResponse{}, fmt.Errorf("streaming message failed: %w", err)
	}

	if opts.StreamHandler != nil {
		opts.StreamHandler.HandleMessageDone()
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
