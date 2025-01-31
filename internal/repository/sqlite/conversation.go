package sqlite

import (
	"context"

	"github.com/google/uuid"
	"github.com/isaacphi/slop/internal/domain"
	"github.com/isaacphi/slop/internal/repository"

	"gorm.io/gorm"
)

type conversationRepo struct {
	db *gorm.DB
}

func NewConversationRepository(db *gorm.DB) repository.ConversationRepository {
	return &conversationRepo{db: db}
}

func (r *conversationRepo) Create(ctx context.Context, conv *domain.Conversation) error {
	return r.db.WithContext(ctx).Create(conv).Error
}

func (r *conversationRepo) GetByID(ctx context.Context, id uint) (*domain.Conversation, error) {
	var conv domain.Conversation
	if err := r.db.WithContext(ctx).Preload("Messages").First(&conv, id).Error; err != nil {
		return nil, err
	}
	return &conv, nil
}

func (r *conversationRepo) List(ctx context.Context) ([]*domain.Conversation, error) {
	var convs []*domain.Conversation
	if err := r.db.WithContext(ctx).Find(&convs).Error; err != nil {
		return nil, err
	}
	return convs, nil
}

func (r *conversationRepo) AddMessage(ctx context.Context, convID uuid.UUID, msg *domain.Message) error {
	msg.ConversationID = convID
	return r.db.WithContext(ctx).Create(msg).Error
}
