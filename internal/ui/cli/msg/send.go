package msg

import (
	"fmt"

	"github.com/isaacphi/wheel/internal/config"
	"github.com/isaacphi/wheel/internal/domain"
	sqliteRepo "github.com/isaacphi/wheel/internal/repository/sqlite"
	"github.com/isaacphi/wheel/internal/service"
	"github.com/spf13/cobra"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var sendCmd = &cobra.Command{
	Use:   "send [message]",
	Short: "Send a single message",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load the configuration
		cfg, err := config.New(true)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		fmt.Println("Config:", cfg)

		// Initialize the database connection
		db, err := gorm.Open(sqlite.Open(cfg.GetString("DBPath")), &gorm.Config{})
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}

		// AutoMigrate
		err = db.AutoMigrate(&domain.Conversation{}, &domain.Message{})
		if err != nil {
			return err
		}

		// Create the repositories and services
		conversationRepo := sqliteRepo.NewConversationRepository(db)
		chatService := service.NewChatService(conversationRepo)

		// Create a new conversation
		conversation, err := chatService.NewConversation(cmd.Context())
		if err != nil {
			return fmt.Errorf("failed to create new conversation: %w", err)
		}

		// Send the message and stream the response
		fmt.Printf("You: %s\n", args[0])
		err = chatService.SendMessageStream(cmd.Context(), conversation.ID, args[0], func(chunk string) error {
			fmt.Print(chunk)
			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to send message: %w", err)
		}
		fmt.Println()
		return nil
	},
}
