package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/isaacphi/slop/internal/domain"
)

type ConversationRepository interface {
	Create(ctx context.Context, conv *domain.Conversation) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Conversation, error)
	List(ctx context.Context) ([]*domain.Conversation, error)
	AddMessage(ctx context.Context, convID uuid.UUID, msg *domain.Message) error
	GetMostRecent(ctx context.Context) (*domain.Conversation, error)
}
