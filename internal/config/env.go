package config

import (
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

func loadEnv() error {
	// Try to load from .env file
	if err := godotenv.Load(); err != nil {
		// If .env doesn't exist, try to load from user's home directory
		home, err := os.UserHomeDir()
		if err == nil {
			godotenv.Load(filepath.Join(home, ".wheel.env"))
		}
	}

	// Set OpenAI API key from environment
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey != "" {
		viper.Set("openai.api_key", apiKey)
	}

	return nil
}

