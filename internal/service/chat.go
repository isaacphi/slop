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
	modelCfg, ok := cfg.Models[cfg.ActiveModel]
	if !ok {
		return nil, fmt.Errorf("model %s not found in configuration", cfg.ActiveModel)
	}

	llmClient, err := llm.NewClient(&modelCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM client: %w", err)
	}

	return &ChatService{
		threadRepo: repo,
		llm:        llmClient,
	}, nil
}

type SendMessageOptions struct {
	ThreadID       uuid.UUID
	ParentID       *uuid.UUID // Optional: message to reply to. If nil, starts a new conversation
	Content        string
	Stream         bool
	StreamCallback func(chunk string) error // Required if Stream is true
}

func (s *ChatService) SendMessage(ctx context.Context, opts SendMessageOptions) (*domain.Message, error) {
	// Verify thread exists
	thread, err := s.threadRepo.GetThreadByID(ctx, opts.ThreadID)
	if err != nil {
		return nil, fmt.Errorf("failed to get thread: %w", err)
	}

	// If no parent specified, get the most recent message in thread
	if opts.ParentID == nil {
		messages, err := s.threadRepo.GetMessages(ctx, thread.ID, nil, false)
		if err != nil {
			return nil, fmt.Errorf("failed to get messages: %w", err)
		}
		if len(messages) > 0 {
			lastMsg := messages[len(messages)-1]
			opts.ParentID = &lastMsg.ID
		}
	}

	// Get conversation history for context
	messages, err := s.threadRepo.GetMessages(ctx, thread.ID, opts.ParentID, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get conversation history: %w", err)
	}

	// Create user message
	modelCfg := s.llm.GetConfig()
	userMsg := &domain.Message{
		ThreadID: opts.ThreadID,
		ParentID: opts.ParentID,
		Role:     domain.RoleHuman,
		Content:  opts.Content,
	}

	// Get AI response
	var aiResponse string
	if opts.Stream {
		var fullResponse strings.Builder
		_, err = s.llm.Chat(ctx, opts.Content, messages, true, func(chunk []byte) error {
			chunkStr := string(chunk)
			fullResponse.WriteString(chunkStr)
			if err := opts.StreamCallback(chunkStr); err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("failed to stream AI response: %w", err)
		}
		aiResponse = fullResponse.String()
	} else {
		aiResponse, err = s.llm.Chat(ctx, opts.Content, messages, false, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to get AI response: %w", err)
		}
	}

	// Create AI message as a reply to the user message
	aiMsg := &domain.Message{
		ThreadID:  opts.ThreadID,
		ParentID:  &userMsg.ID, // AI message is a child of the user message
		Role:      domain.RoleAssistant,
		Content:   aiResponse,
		ModelName: modelCfg.Name,
		Provider:  modelCfg.Provider,
	}

	if err := s.threadRepo.AddMessageToThread(ctx, opts.ThreadID, userMsg); err != nil {
		return nil, err
	}
	if err := s.threadRepo.AddMessageToThread(ctx, opts.ThreadID, aiMsg); err != nil {
		return nil, err
	}

	return aiMsg, nil
}

func (s *ChatService) NewThread(ctx context.Context) (*domain.Thread, error) {
	thread := &domain.Thread{}
	return thread, s.threadRepo.CreateThread(ctx, thread)
}

func (s *ChatService) GetActiveThread(ctx context.Context) (*domain.Thread, error) {
	thread, err := s.threadRepo.GetMostRecentThread(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get most recent thread: %w", err)
	}
	return thread, nil
}

// ListThreads returns a list of threads, optionally limited to a specific number
func (s *ChatService) ListThreads(ctx context.Context, limit int) ([]*domain.Thread, error) {
	return s.threadRepo.ListThreads(ctx, limit)
}

// FindThreadByPartialID finds a thread by a partial ID string
func (s *ChatService) FindThreadByPartialID(ctx context.Context, partialID string) (*domain.Thread, error) {
	return s.threadRepo.GetThreadByPartialID(ctx, partialID)
}

// GetThreadDetails returns a brief summary of a thread for display purposes
type ThreadDetails struct {
	ID           uuid.UUID
	CreatedAt    time.Time
	MessageCount int
	Preview      string
}

func (s *ChatService) SetThreadSummary(ctx context.Context, thread *domain.Thread, summary string) error {
	return s.threadRepo.SetThreadSummary(ctx, thread.ID, summary)
}

func (s *ChatService) GetThreadDetails(ctx context.Context, thread *domain.Thread) (*ThreadDetails, error) {
	messages, err := s.threadRepo.GetMessages(ctx, thread.ID, nil, false)
	if err != nil {
		return nil, err
	}

	preview := ""
	if thread.Summary != "" {
		preview = thread.Summary
	} else {
		for _, msg := range messages {
			if msg.Role == domain.RoleHuman {
				preview = msg.Content
				break
			}
		}
	}
	if len(preview) > 50 {
		preview = preview[:47] + "..."
	}

	return &ThreadDetails{
		ID:           thread.ID,
		CreatedAt:    thread.CreatedAt,
		MessageCount: len(messages),
		Preview:      preview,
	}, nil
}

// DeleteThread deletes a thread and all its messages
func (s *ChatService) DeleteThread(ctx context.Context, threadID uuid.UUID) error {
	// Check if thread exists first
	if _, err := s.threadRepo.GetThreadByID(ctx, threadID); err != nil {
		return fmt.Errorf("failed to find thread: %w", err)
	}

	return s.threadRepo.DeleteThread(ctx, threadID)
}

// GetThreadMessages returns all messages in a thread
func (s *ChatService) GetThreadMessages(ctx context.Context, threadID uuid.UUID, messageID *uuid.UUID) ([]domain.Message, error) {
	return s.threadRepo.GetMessages(ctx, threadID, messageID, true)
}

// DeleteLastMessages deletes the specified number of most recent messages from a thread
func (s *ChatService) DeleteLastMessages(ctx context.Context, threadID uuid.UUID, count int) error {
	return s.threadRepo.DeleteLastMessages(ctx, threadID, count)
}

func (s *ChatService) FindMessageByPartialID(ctx context.Context, threadID uuid.UUID, partialID string) (*domain.Message, error) {
	if _, err := s.threadRepo.GetThreadByID(ctx, threadID); err != nil {
		return nil, fmt.Errorf("thread not found: %w", err)
	}

	return s.threadRepo.FindMessageByPartialID(ctx, threadID, partialID)
}
