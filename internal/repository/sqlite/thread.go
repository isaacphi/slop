package sqlite

import (
	"context"

	"github.com/google/uuid"
	"github.com/isaacphi/slop/internal/domain"
	"github.com/isaacphi/slop/internal/repository"

	"gorm.io/gorm"
)

type threadRepo struct {
	db *gorm.DB
}

func NewThreadRepository(db *gorm.DB) repository.ThreadRepository {
	return &threadRepo{db: db}
}

func (r *threadRepo) Create(ctx context.Context, thread *domain.Thread) error {
	return r.db.WithContext(ctx).Create(thread).Error
}

func (r *threadRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Thread, error) {
	var thread domain.Thread
	if err := r.db.WithContext(ctx).Preload("Messages").First(&thread, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.NoConversationError{}
		}
		return nil, err
	}
	return &thread, nil
}

func (r *threadRepo) List(ctx context.Context) ([]*domain.Thread, error) {
	var threads []*domain.Thread
	if err := r.db.WithContext(ctx).Find(&threads).Error; err != nil {
		return nil, err
	}
	return threads, nil
}

func (r *threadRepo) GetMostRecent(ctx context.Context) (*domain.Thread, error) {
	var thread domain.Thread
	if err := r.db.WithContext(ctx).Order("created_at DESC").First(&thread).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.NoConversationError{}
		}
		return nil, err
	}
	return &thread, nil
}

func (r *threadRepo) AddMessage(ctx context.Context, threadID uuid.UUID, msg *domain.Message) error {
	msg.ThreadID = threadID
	return r.db.WithContext(ctx).Create(msg).Error
}

func (r *threadRepo) GetMessages(ctx context.Context, threadID uuid.UUID) ([]domain.Message, error) {
	var messages []domain.Message
	if err := r.db.WithContext(ctx).Where("thread_id = ?", threadID).Order("created_at ASC").Find(&messages).Error; err != nil {
		return nil, err
	}
	return messages, nil
}
