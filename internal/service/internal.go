package service

import (
	"context"
	"fmt"

	"github.com/isaacphi/slop/internal/config"
	"github.com/isaacphi/slop/internal/domain"
	"github.com/isaacphi/slop/internal/llm"
)

type InternalService struct {
	llm *llm.Client
	cfg *config.Internal
}

func NewInternalService() (*InternalService, error) {
	cfg, err := config.New()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	modelCfg, ok := cfg.Models[cfg.Internal.Model]
	if !ok {
		return nil, fmt.Errorf("model %s not found in configuration", cfg.ActiveModel)
	}

	llmClient, err := llm.NewClient(&modelCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM client for internal service: %w", err)
	}

	return &InternalService{
		llm: llmClient,
		cfg: &cfg.Internal,
	}, nil
}

// ChatOneOff makes a single call to the LLM without storing any context or history
func (s *InternalService) GenerateOneOff(ctx context.Context, prompt string) (string, error) {
	response, err := s.llm.Chat(ctx, prompt, []domain.Message{})
	if err != nil {
		return "", fmt.Errorf("internal chat failed: %w", err)
	}
	return response, nil
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
