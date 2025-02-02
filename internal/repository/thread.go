package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/isaacphi/slop/internal/domain"
)

type ThreadRepository interface {
	Create(ctx context.Context, thread *domain.Thread) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Thread, error)
	List(ctx context.Context, limit int) ([]*domain.Thread, error)
	AddMessage(ctx context.Context, threadID uuid.UUID, msg *domain.Message) error
	GetMostRecent(ctx context.Context) (*domain.Thread, error)
	GetMessages(ctx context.Context, threadID uuid.UUID, messageID *uuid.UUID) ([]domain.Message, error)
	FindByPartialID(ctx context.Context, partialID string) (*domain.Thread, error)
	Delete(ctx context.Context, id uuid.UUID) error
	DeleteLastMessages(ctx context.Context, threadID uuid.UUID, count int) error
	SetThreadSummary(ctx context.Context, threadId uuid.UUID, summary string) error
}
