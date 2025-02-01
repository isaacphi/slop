package service

import (
	"context"
	"fmt"
	"strings"
	"time"

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

// ListThreads returns a list of threads, optionally limited to a specific number
func (s *ChatService) ListThreads(ctx context.Context, limit int) ([]*domain.Thread, error) {
	return s.threadRepo.List(ctx, limit)
}

// FindThreadByPartialID finds a thread by a partial ID string
func (s *ChatService) FindThreadByPartialID(ctx context.Context, partialID string) (*domain.Thread, error) {
	return s.threadRepo.FindByPartialID(ctx, partialID)
}

// GetThreadSummary returns a brief summary of a thread for display purposes
type ThreadSummary struct {
	ID           uuid.UUID
	CreatedAt    time.Time
	MessageCount int
	Preview      string
}

func (s *ChatService) GetThreadSummary(ctx context.Context, thread *domain.Thread) (*ThreadSummary, error) {
	messages, err := s.threadRepo.GetMessages(ctx, thread.ID)
	if err != nil {
		return nil, err
	}

	// Get preview from first human message
	preview := ""
	for _, msg := range messages {
		if msg.Role == domain.RoleHuman {
			preview = msg.Content
			if len(preview) > 50 {
				preview = preview[:47] + "..."
			}
			break
		}
	}

	return &ThreadSummary{
		ID:           thread.ID,
		CreatedAt:    thread.CreatedAt,
		MessageCount: len(messages),
		Preview:      preview,
	}, nil
}

// DeleteThread deletes a thread and all its messages
func (s *ChatService) DeleteThread(ctx context.Context, threadID uuid.UUID) error {
	// Check if thread exists first
	if _, err := s.threadRepo.GetByID(ctx, threadID); err != nil {
		return fmt.Errorf("failed to find thread: %w", err)
	}

	return s.threadRepo.Delete(ctx, threadID)
}
