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
	modelCfg config.ModelPreset
	cfg      config.Internal
}

func NewInternalService(cfg *config.ConfigSchema) (*InternalService, error) {
	modelCfg, ok := cfg.ModelPresets[cfg.Internal.Model]
	if !ok {
		return nil, fmt.Errorf("model %s not found in configuration", cfg.ActiveModel)
	}

	return &InternalService{
		modelCfg: modelCfg,
		cfg:      cfg.Internal,
	}, nil
}

// GenerateOneOff makes a single call to the LLM without storing any context or history
func (s *InternalService) GenerateOneOff(ctx context.Context, prompt string) (string, error) {
	response, err := llm.GenerateContent(ctx, s.modelCfg, prompt, []domain.Message{}, nil, nil)
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
