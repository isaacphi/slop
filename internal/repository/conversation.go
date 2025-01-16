package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/isaacphi/wheel/internal/domain"
)

type ConversationRepository interface {
	Create(ctx context.Context, conv *domain.Conversation) error
	GetByID(ctx context.Context, id uint) (*domain.Conversation, error)
	List(ctx context.Context) ([]*domain.Conversation, error)
	AddMessage(ctx context.Context, convID uuid.UUID, msg *domain.Message) error
}
