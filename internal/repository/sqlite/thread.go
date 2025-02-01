package sqlite

import (
	"context"
	"strings"

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

func (r *threadRepo) Delete(ctx context.Context, id uuid.UUID) error {
	// Start a transaction to ensure all related records are deleted
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Delete all messages associated with the thread
		if err := tx.Where("thread_id = ?", id).Delete(&domain.Message{}).Error; err != nil {
			return err
		}

		// Delete the thread itself
		if err := tx.Delete(&domain.Thread{}, id).Error; err != nil {
			return err
		}

		return nil
	})
}

func (r *threadRepo) List(ctx context.Context, limit int) ([]*domain.Thread, error) {
	var threads []*domain.Thread
	query := r.db.WithContext(ctx).Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := query.Find(&threads).Error; err != nil {
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

func (r *threadRepo) FindByPartialID(ctx context.Context, partialID string) (*domain.Thread, error) {
	var thread domain.Thread

	// Convert the string to lowercase for case-insensitive comparison
	partialID = strings.ToLower(partialID)

	// Find threads where the ID starts with the partial ID
	if err := r.db.WithContext(ctx).
		Preload("Messages").
		Where("LOWER(CAST(id AS TEXT)) LIKE ?", partialID+"%").
		Order("created_at DESC").
		First(&thread).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.NoConversationError{}
		}
		return nil, err
	}

	return &thread, nil
}

func (r *threadRepo) DeleteLastMessages(ctx context.Context, threadID uuid.UUID, count int) error {
	// Get the IDs of the last 'count' messages
	var messageIDs []uuid.UUID
	if err := r.db.WithContext(ctx).
		Model(&domain.Message{}).
		Where("thread_id = ?", threadID).
		Order("created_at DESC").
		Limit(count).
		Pluck("id", &messageIDs).Error; err != nil {
		return err
	}

	// Delete the messages
	if len(messageIDs) > 0 {
		if err := r.db.WithContext(ctx).
			Where("id IN ?", messageIDs).
			Delete(&domain.Message{}).Error; err != nil {
			return err
		}
	}

	return nil
}
