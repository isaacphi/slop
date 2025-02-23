package sqlite

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/isaacphi/slop/internal/domain"
	"gorm.io/gorm"
)

func (r *messageRepo) CreateThread(ctx context.Context, thread *domain.Thread) error {
	return r.db.WithContext(ctx).Create(thread).Error
}

func (r *messageRepo) GetThread(ctx context.Context, id uuid.UUID) (*domain.Thread, error) {
	var thread domain.Thread
	if err := r.db.WithContext(ctx).
		Preload("Messages").
		First(&thread, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("Conversation not found")
		}
		return nil, err
	}
	return &thread, nil
}

func (r *messageRepo) DeleteThread(ctx context.Context, id uuid.UUID) error {
	// Start a transaction to ensure all related records are deleted
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Delete all messages associated with the thread
		if err := tx.Where("thread_id = ?", id).Delete(&domain.Message{}).Error; err != nil {
			return err
		}
		return tx.Delete(&domain.Thread{}, id).Error
	})
}

func (r *messageRepo) ListThreads(ctx context.Context, limit int) ([]*domain.Thread, error) {
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

func (r *messageRepo) GetMostRecentThread(ctx context.Context) (*domain.Thread, error) {
	var thread domain.Thread
	if err := r.db.WithContext(ctx).Order("created_at DESC").First(&thread).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("Conversation not found")
		}
		return nil, err
	}
	return &thread, nil
}

func (r *messageRepo) GetThreadByPartialID(ctx context.Context, partialID string) (*domain.Thread, error) {
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
			return nil, fmt.Errorf("Conversation not found")
		}
		return nil, err
	}

	return &thread, nil
}

func (r *messageRepo) SetThreadSummary(ctx context.Context, threadId uuid.UUID, summary string) error {
	return r.db.WithContext(ctx).Model(&domain.Thread{}).Where("id = ?", threadId).Update("summary", summary).Error
}
