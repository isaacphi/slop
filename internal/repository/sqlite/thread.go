package sqlite

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

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
	if err := r.db.WithContext(ctx).
		Preload("Messages").
		First(&thread, id).Error; err != nil {
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
		return tx.Delete(&domain.Thread{}, id).Error
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

func (r *threadRepo) GetMessages(ctx context.Context, threadID uuid.UUID, messageID *uuid.UUID) ([]domain.Message, error) {
	var messages []domain.Message

	// Load all messages with their relationships
	if err := r.db.WithContext(ctx).
		Where("thread_id = ?", threadID).
		Preload("Parent").
		Preload("Children").
		Find(&messages).Error; err != nil {
		return nil, err
	}

	// Build a map for easier lookup
	messageMap := make(map[uuid.UUID]*domain.Message)
	for i := range messages {
		messageMap[messages[i].ID] = &messages[i]
	}

	// Find our starting message
	var startMessage *domain.Message
	if messageID == nil {
		var newest time.Time
		for i := range messages {
			if messages[i].CreatedAt.After(newest) {
				newest = messages[i].CreatedAt
				startMessage = &messages[i]
			}
		}
	} else {
		var exists bool
		startMessage, exists = messageMap[*messageID]
		if !exists {
			return nil, fmt.Errorf("message %s not found", messageID)
		}
	}

	// Collect messages in the branch
	branchMessages := make(map[uuid.UUID]domain.Message)

	// Work backwards to collect all parents
	current := startMessage
	for current != nil {
		branchMessages[current.ID] = *current
		if current.ParentID == nil {
			break
		}
		current = messageMap[*current.ParentID]
	}

	// Work forwards from startMessage to get the newest child path
	current = startMessage
	for current != nil {
		// If no children, we're done
		if len(current.Children) == 0 {
			break
		}

		// Find newest child
		var newestChild *domain.Message
		var newestTime time.Time
		for i := range current.Children {
			child := &current.Children[i]
			if child.CreatedAt.After(newestTime) {
				newestTime = child.CreatedAt
				newestChild = child
			}
		}

		// Add newest child to our branch and continue with that child
		branchMessages[newestChild.ID] = *newestChild
		current = newestChild
	}

	// Convert map to slice and sort by creation time
	result := make([]domain.Message, 0, len(branchMessages))
	for _, msg := range branchMessages {
		result = append(result, msg)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.Before(result[j].CreatedAt)
	})

	return result, nil
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

func (r *threadRepo) SetThreadSummary(ctx context.Context, threadId uuid.UUID, summary string) error {
	return r.db.WithContext(ctx).Model(&domain.Thread{}).Where("id = ?", threadId).Update("summary", summary).Error
}
