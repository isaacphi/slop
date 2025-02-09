package sqlite

import (
	"github.com/isaacphi/slop/internal/repository"

	"gorm.io/gorm"
)

type threadRepo struct {
	db *gorm.DB
}

func NewThreadRepository(db *gorm.DB) repository.ThreadRepository {
	return &threadRepo{db: db}
}
