package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/isaacphi/slop/internal/domain"
)

type ThreadRepository interface {
	CreateThread(ctx context.Context, thread *domain.Thread) error
	GetThreadByID(ctx context.Context, id uuid.UUID) (*domain.Thread, error)
	ListThreads(ctx context.Context, limit int) ([]*domain.Thread, error)
	AddMessageToThread(ctx context.Context, threadID uuid.UUID, msg *domain.Message) error
	GetMostRecentThread(ctx context.Context) (*domain.Thread, error)
	GetMessages(ctx context.Context, threadID uuid.UUID, messageID *uuid.UUID, getFutureMessages bool) ([]domain.Message, error)
	GetThreadByPartialID(ctx context.Context, partialID string) (*domain.Thread, error)
	DeleteThread(ctx context.Context, id uuid.UUID) error
	DeleteLastMessages(ctx context.Context, threadID uuid.UUID, count int) error
	SetThreadSummary(ctx context.Context, threadId uuid.UUID, summary string) error
	FindMessageByPartialID(ctx context.Context, threadID uuid.UUID, partialID string) (*domain.Message, error)
}
