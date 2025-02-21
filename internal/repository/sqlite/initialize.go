package sqlite

import (
	"fmt"

	"github.com/isaacphi/slop/internal/domain"
	"github.com/isaacphi/slop/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Initialize creates a new SQLite message repository with the given database path
func Initialize(dbPath string) (repository.MessageRepository, error) {
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Run migrations
	if err := db.AutoMigrate(&domain.Thread{}, &domain.Message{}); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return NewMessageRepository(db), nil
}
