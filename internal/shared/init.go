package shared

import (
	"fmt"

	"github.com/isaacphi/slop/internal/config"
	"github.com/isaacphi/slop/internal/domain"
	sqliteRepo "github.com/isaacphi/slop/internal/repository/sqlite"
	"github.com/isaacphi/slop/internal/service"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func InitializeChatService(modelOverride string) (*service.ChatService, error) {
	// Load the configuration
	cfg, err := config.New(false)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Override model if specified
	if modelOverride != "" {
		cfg.ActiveModel = modelOverride
	}

	// Initialize the database connection
	db, err := gorm.Open(sqlite.Open(cfg.DBPath), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// AutoMigrate
	err = db.AutoMigrate(&domain.Thread{}, &domain.Message{})
	if err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	// Create the repositories and services
	threadRepo := sqliteRepo.NewThreadRepository(db)
	chatService, err := service.NewChatService(threadRepo, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat service: %w", err)
	}

	return chatService, nil
}

