package message

import (
	"fmt"

	"github.com/isaacphi/slop/internal/config"
	"github.com/isaacphi/slop/internal/domain"
	sqliteRepo "github.com/isaacphi/slop/internal/repository/sqlite"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// InitializeMessageService creates and initializes the message service with all required dependencies
func InitializeMessageService(cfg *config.ConfigSchema, overrides *MessageServiceOverrides) (*MessageService, error) {
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
	threadRepo := sqliteRepo.NewMessageRepository(db)

	modelName := cfg.ActiveModel
	if overrides != nil {
		if overrides.ActiveModel != nil {
			modelName = *overrides.ActiveModel
		}
	}
	modelConfig, exists := cfg.ModelPresets[modelName]
	if !exists {
		return nil, fmt.Errorf("Model %s not found in config", modelName)
	}
	if overrides != nil {
		if overrides.MaxTokens != nil {
			modelConfig.MaxTokens = *overrides.MaxTokens
		}
		if overrides.Temperature != nil {
			modelConfig.Temperature = *overrides.Temperature
		}
	}

	messageService, err := New(threadRepo, modelConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create message service: %w", err)
	}

	return messageService, nil
}
