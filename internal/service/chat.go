package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/isaacphi/slop/internal/config"
	"github.com/isaacphi/slop/internal/domain"
	"github.com/isaacphi/slop/internal/llm"
	"github.com/isaacphi/slop/internal/repository"

	"github.com/google/uuid"
)

type ChatService struct {
	threadRepo repository.ThreadRepository
	llm        *llm.Client
}

func NewChatService(repo repository.ThreadRepository, cfg *config.ConfigSchema) (*ChatService, error) {
	llmClient, err := llm.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM client: %w", err)
	}

	return &ChatService{
		threadRepo: repo,
		llm:        llmClient,
	}, nil
}

func (s *ChatService) SendMessage(ctx context.Context, threadID uuid.UUID, content string) (*domain.Message, error) {
	thread, err := s.threadRepo.GetByID(ctx, threadID)
	if err != nil {
		return nil, fmt.Errorf("failed to get thread: %w", err)
	}

	modelCfg := s.llm.GetConfig()
	userMsg := &domain.Message{
		ThreadID: threadID,
		Role:     domain.RoleHuman,
		Content:  content,
	}

	response, err := s.llm.Chat(ctx, content, thread.Messages)
	if err != nil {
		return nil, err
	}

	aiMsg := &domain.Message{
		ThreadID:  threadID,
		Role:      domain.RoleAssistant,
		Content:   response,
		ModelName: modelCfg.Name,
		Provider:  modelCfg.Provider,
	}

	if err := s.threadRepo.AddMessage(ctx, threadID, userMsg); err != nil {
		return nil, err
	}
	if err := s.threadRepo.AddMessage(ctx, threadID, aiMsg); err != nil {
		return nil, err
	}

	return aiMsg, nil
}

func (s *ChatService) NewThread(ctx context.Context) (*domain.Thread, error) {
	thread := &domain.Thread{}
	return thread, s.threadRepo.Create(ctx, thread)
}

func (s *ChatService) GetActiveThread(ctx context.Context) (*domain.Thread, error) {
	thread, err := s.threadRepo.GetMostRecent(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get most recent thread: %w", err)
	}
	return thread, nil
}

func (s *ChatService) SendMessageStream(ctx context.Context, threadID uuid.UUID, content string, callback func(chunk string) error) error {
	thread, err := s.threadRepo.GetByID(ctx, threadID)
	if err != nil {
		return fmt.Errorf("failed to get thread: %w", err)
	}

	modelCfg := s.llm.GetConfig()
	userMsg := &domain.Message{
		ThreadID: threadID,
		Role:     domain.RoleHuman,
		Content:  content,
	}

	if err := s.threadRepo.AddMessage(ctx, threadID, userMsg); err != nil {
		return fmt.Errorf("failed to store user message: %w", err)
	}

	var fullResponse strings.Builder
	err = s.llm.ChatStream(ctx, content, thread.Messages, func(chunk []byte) error {
		chunkStr := string(chunk)
		fullResponse.WriteString(chunkStr)
		if err := callback(chunkStr); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to stream AI response: %w", err)
	}

	aiMsg := &domain.Message{
		ThreadID:  threadID,
		Role:      domain.RoleAssistant,
		Content:   fullResponse.String(),
		ModelName: modelCfg.Name,
		Provider:  modelCfg.Provider,
	}

	if err := s.threadRepo.AddMessage(ctx, threadID, aiMsg); err != nil {
		return fmt.Errorf("failed to store AI message: %w", err)
	}

	return nil
}
