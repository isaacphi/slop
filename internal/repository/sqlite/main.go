package sqlite

import (
	"github.com/isaacphi/slop/internal/repository"

	"gorm.io/gorm"
)

type messageRepo struct {
	db *gorm.DB
}

func NewMessageRepository(db *gorm.DB) repository.MessageRepository {
	return &messageRepo{db: db}
}
