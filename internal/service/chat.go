package service

import (
	"context"
	"fmt"
	"log"

	"github.com/isaacphi/wheel/internal/domain"
	"github.com/isaacphi/wheel/internal/llm"
	"github.com/isaacphi/wheel/internal/repository"

	"github.com/google/uuid"
)

type ChatService struct {
	convRepo repository.ConversationRepository
	llm      *llm.Client
}

func NewChatService(repo repository.ConversationRepository) *ChatService {
	llmClient, err := llm.NewClient(nil)
	if err != nil {
		log.Fatalf("failed to create LLM client: %v", err)
	}

	return &ChatService{
		convRepo: repo,
		llm:      llmClient,
	}
}

func (s *ChatService) SendMessage(ctx context.Context, convID uuid.UUID, content string) (*domain.Message, error) {
	userMsg := &domain.Message{
		ConversationID: convID,
		Role:           "human",
		Content:        content,
	}

	response, err := s.llm.Chat(ctx, content)
	if err != nil {
		return nil, err
	}

	aiMsg := &domain.Message{
		ConversationID: convID,
		Role:           "assistant",
		Content:        response,
	}

	if err := s.convRepo.AddMessage(ctx, convID, userMsg); err != nil {
		return nil, err
	}
	if err := s.convRepo.AddMessage(ctx, convID, aiMsg); err != nil {
		return nil, err
	}

	return aiMsg, nil
}

func (s *ChatService) NewConversation(ctx context.Context) (*domain.Conversation, error) {
	conv := &domain.Conversation{}
	return conv, s.convRepo.Create(ctx, conv)
}

func (s *ChatService) SendMessageStream(ctx context.Context, convID uuid.UUID, content string, callback func(chunk string) error) error {
	userMsg := &domain.Message{
		ConversationID: convID,
		Role:           "human",
		Content:        content,
	}

	if err := s.convRepo.AddMessage(ctx, convID, userMsg); err != nil {
		return fmt.Errorf("failed to store user message: %w", err)
	}

	err := s.llm.ChatStream(ctx, content, func(chunk []byte) error {
		chunkStr := string(chunk)
		if err := callback(chunkStr); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to stream AI response: %w", err)
	}

	fullResponse := ""
	err = s.llm.ChatStream(ctx, content, func(chunk []byte) error {
		fullResponse += string(chunk)
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to collect AI response: %w", err)
	}

	aiMsg := &domain.Message{
		ConversationID: convID,
		Role:           "assistant",
		Content:        fullResponse,
	}

	if err := s.convRepo.AddMessage(ctx, convID, aiMsg); err != nil {
		return fmt.Errorf("failed to store AI message: %w", err)
	}

	return nil
}
