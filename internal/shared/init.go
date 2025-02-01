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

// ServiceOptions holds the configuration options for service initialization
type ServiceOptions struct {
	Model       string  // Model override
	MaxTokens   int     // Max tokens override
	Temperature float64 // Temperature override
}

// InitializeChatService creates and initializes the chat service with all required dependencies
func InitializeChatService(opts *ServiceOptions) (*service.ChatService, error) {
	// Build runtime overrides from options
	var overrides *config.RuntimeOverrides
	if opts != nil {
		overrides = &config.RuntimeOverrides{}
		if opts.Model != "" {
			overrides.ActiveModel = &opts.Model
		}
		if opts.MaxTokens > 0 {
			overrides.MaxTokens = &opts.MaxTokens
		}
		if opts.Temperature > 0 {
			overrides.Temperature = &opts.Temperature
		}
	}

	// Load the configuration with overrides
	cfg, err := config.NewConfigWithOverrides(overrides)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
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

