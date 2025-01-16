package service

import (
	"context"
	"github.com/isaacphi/wheel/internal/domain"
	"github.com/isaacphi/wheel/internal/llm"
	"github.com/isaacphi/wheel/internal/repository"

	"github.com/google/uuid"
)

type ChatService struct {
	convRepo repository.ConversationRepository
	llm      *llm.Client
}

func NewChatService(repo repository.ConversationRepository, llm *llm.Client) *ChatService {
	return &ChatService{
		convRepo: repo,
		llm:      llm,
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
