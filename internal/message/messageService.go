package message

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/isaacphi/slop/internal/config"
	"github.com/isaacphi/slop/internal/domain"
	"github.com/isaacphi/slop/internal/llm"
	"github.com/isaacphi/slop/internal/repository"
	sqliteRepo "github.com/isaacphi/slop/internal/repository/sqlite"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type MessageService struct {
	messageRepo repository.MessageRepository
	llm         *llm.Client
}

func New(repo repository.MessageRepository, modelCfg config.Model) (*MessageService, error) {

	llmClient, err := llm.NewClient(modelCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM client: %w", err)
	}

	return &MessageService{
		messageRepo: repo,
		llm:         llmClient,
	}, nil
}

type SendMessageOptions struct {
	ThreadID       uuid.UUID
	ParentID       *uuid.UUID // Optional: message to reply to. If nil, starts a new conversation
	Content        string
	StreamCallback func(chunk []byte) error
	Tools          map[string]config.Tool
}

func (s *MessageService) SendMessage(ctx context.Context, opts SendMessageOptions) (*domain.Message, error) {
	// Verify thread exists
	thread, err := s.messageRepo.GetThreadByID(ctx, opts.ThreadID)
	if err != nil {
		return nil, fmt.Errorf("failed to get thread: %w", err)
	}

	// If no parent specified, get the most recent message in thread
	if opts.ParentID == nil {
		messages, err := s.messageRepo.GetMessages(ctx, thread.ID, nil, false)
		if err != nil {
			return nil, fmt.Errorf("failed to get messages: %w", err)
		}
		if len(messages) > 0 {
			lastMsg := messages[len(messages)-1]
			opts.ParentID = &lastMsg.ID
		}
	}

	// Get conversation history for context
	messages, err := s.messageRepo.GetMessages(ctx, thread.ID, opts.ParentID, false)
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
	aiResponse, err := s.llm.SendMessage(ctx, opts.Content, messages, opts.StreamCallback != nil, opts.StreamCallback, opts.Tools)
	if err != nil {
		return nil, fmt.Errorf("failed to stream AI response: %w", err)
	}

	toolCallsString, err := json.Marshal(aiResponse.ToolCalls)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse ToolCalls: %w", err)
	}

	// Create AI message as a reply to the user message
	aiMsg := &domain.Message{
		ThreadID:  opts.ThreadID,
		ParentID:  &userMsg.ID, // AI message is a child of the user message
		Role:      domain.RoleAssistant,
		Content:   aiResponse.TextResponse,
		ToolCalls: string(toolCallsString),
		ModelName: modelCfg.Name,
		Provider:  modelCfg.Provider,
	}

	if err := s.messageRepo.AddMessageToThread(ctx, opts.ThreadID, userMsg); err != nil {
		return nil, err
	}
	if err := s.messageRepo.AddMessageToThread(ctx, opts.ThreadID, aiMsg); err != nil {
		return nil, err
	}

	return aiMsg, nil
}

func (s *MessageService) NewThread(ctx context.Context) (*domain.Thread, error) {
	thread := &domain.Thread{}
	return thread, s.messageRepo.CreateThread(ctx, thread)
}

func (s *MessageService) GetActiveThread(ctx context.Context) (*domain.Thread, error) {
	thread, err := s.messageRepo.GetMostRecentThread(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get most recent thread: %w", err)
	}
	return thread, nil
}

// ListThreads returns a list of threads, optionally limited to a specific number
func (s *MessageService) ListThreads(ctx context.Context, limit int) ([]*domain.Thread, error) {
	return s.messageRepo.ListThreads(ctx, limit)
}

// FindThreadByPartialID finds a thread by a partial ID string
func (s *MessageService) FindThreadByPartialID(ctx context.Context, partialID string) (*domain.Thread, error) {
	return s.messageRepo.GetThreadByPartialID(ctx, partialID)
}

// GetThreadDetails returns a brief summary of a thread for display purposes
type ThreadDetails struct {
	ID           uuid.UUID
	CreatedAt    time.Time
	MessageCount int
	Preview      string
}

func (s *MessageService) SetThreadSummary(ctx context.Context, thread *domain.Thread, summary string) error {
	return s.messageRepo.SetThreadSummary(ctx, thread.ID, summary)
}

func (s *MessageService) GetThreadDetails(ctx context.Context, thread *domain.Thread) (*ThreadDetails, error) {
	messages, err := s.messageRepo.GetMessages(ctx, thread.ID, nil, false)
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
func (s *MessageService) DeleteThread(ctx context.Context, threadID uuid.UUID) error {
	// Check if thread exists first
	if _, err := s.messageRepo.GetThreadByID(ctx, threadID); err != nil {
		return fmt.Errorf("failed to find thread: %w", err)
	}

	return s.messageRepo.DeleteThread(ctx, threadID)
}

// GetThreadMessages returns all messages in a thread
func (s *MessageService) GetThreadMessages(ctx context.Context, threadID uuid.UUID, messageID *uuid.UUID) ([]domain.Message, error) {
	return s.messageRepo.GetMessages(ctx, threadID, messageID, true)
}

// DeleteLastMessages deletes the specified number of most recent messages from a thread
func (s *MessageService) DeleteLastMessages(ctx context.Context, threadID uuid.UUID, count int) error {
	return s.messageRepo.DeleteLastMessages(ctx, threadID, count)
}

func (s *MessageService) FindMessageByPartialID(ctx context.Context, threadID uuid.UUID, partialID string) (*domain.Message, error) {
	if _, err := s.messageRepo.GetThreadByID(ctx, threadID); err != nil {
		return nil, fmt.Errorf("thread not found: %w", err)
	}

	return s.messageRepo.FindMessageByPartialID(ctx, threadID, partialID)
}

type MessageServiceOverrides struct {
	ActiveModel *string
	MaxTokens   *int
	Temperature *float64
}

// InitializeMessageService creates and initializes the message service with all required dependencies
func InitializeMessageService(cfg *config.ConfigSchema, overrides *MessageServiceOverrides) (*MessageService, error) {
	// Initialize the database connection
	db, err := gorm.Open(sqlite.Open(cfg.DBPath), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// AutoMigrate
	err = db.AutoMigrate(&domain.Thread{}, &domain.Message{})
	if err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	// Create the repositories and services
	threadRepo := sqliteRepo.NewMessageRepository(db)

	modelName := cfg.ActiveModel
	if overrides != nil {
		if overrides.ActiveModel != nil {
			modelName = *overrides.ActiveModel
		}
	}
	modelConfig, exists := cfg.Models[modelName]
	if !exists {
		return nil, fmt.Errorf("Model %s not found in config", modelName)
	}
	if overrides != nil {
		if overrides.MaxTokens != nil {
			modelConfig.MaxTokens = *overrides.MaxTokens
		}
		if overrides.Temperature != nil {
			modelConfig.Temperature = *overrides.Temperature
		}
	}

	messageService, err := New(threadRepo, modelConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create message service: %w", err)
	}

	return messageService, nil
}
