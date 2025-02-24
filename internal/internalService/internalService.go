package internal

import (
	"context"
	"fmt"

	"github.com/isaacphi/slop/internal/config"
	"github.com/isaacphi/slop/internal/domain"
	"github.com/isaacphi/slop/internal/llm"
)

// InternalService is used for LLM calls within the application itself
// such as for summarizing threads
type InternalService struct {
	preset config.Preset
	cfg    config.Internal
}

func NewInternalService(cfg *config.ConfigSchema) (*InternalService, error) {
	preset, ok := cfg.Presets[cfg.Internal.Model]
	if !ok {
		return nil, fmt.Errorf("model %s not found in configuration", cfg.DefaultPreset)
	}

	return &InternalService{
		preset: preset,
		cfg:    cfg.Internal,
	}, nil
}

// GenerateOneOff makes a single call to the LLM without storing any context or history
func (s *InternalService) GenerateOneOff(ctx context.Context, prompt string) (string, error) {
	opts := llm.GenerateContentOptions{
		Preset:  s.preset,
		Content: prompt,
	}
	response, err := llm.GenerateContent(ctx, opts)
	if err != nil {
		return "", fmt.Errorf("internal message failed: %w", err)
	}
	return response.TextResponse, nil
}

// CreateThreadSummary generates a summary for a thread using the internal model
func (s *InternalService) CreateThreadSummary(ctx context.Context, messages []domain.Message) (string, error) {
	if len(messages) == 0 {
		return "[empty]", nil
	}

	prompt := s.cfg.SummaryPrompt

	for _, msg := range messages {
		prompt += fmt.Sprintf("%s: %s\n", msg.Role, msg.Content)
	}

	return s.GenerateOneOff(ctx, prompt)
}
