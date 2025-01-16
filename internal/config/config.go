package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {
	DBPath string `mapstructure:"db_path"`
	LLMKey string `mapstructure:"llm_key"`
}

func Load() (*Config, error) {
	// Load .env file if it exists
	_ = godotenv.Load() // Ignore error as .env file is optional

	viper.SetDefault("db_path", "wheel.db")

	// Check environment variables first
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		viper.SetDefault("llm_key", key)
	}

	// Set up config file locations
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	if home, err := os.UserHomeDir(); err == nil {
		viper.AddConfigPath(filepath.Join(home, ".wheel"))
		viper.AddConfigPath(home)
	}

	// Set up environment variables
	viper.AutomaticEnv()
	viper.SetEnvPrefix("WHEEL") // WHEEL_* environment variables
	viper.BindEnv("llm_key", "OPENAI_API_KEY")

	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	// Validate config
	if cfg.LLMKey == "" {
		return nil, fmt.Errorf("LLM API key not found. Set it via OPENAI_API_KEY environment variable, .env file, or config.yaml")
	}

	return &cfg, nil
}
