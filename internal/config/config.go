package config

import (
	"github.com/spf13/viper"
	"path/filepath"
	"os"
)

type Config struct {
	Database struct {
		Path string
	}
	OpenAI struct {
		APIKey string
	}
}

func Initialize() error {
	// Load environment variables first
	if err := loadEnv(); err != nil {
		return err
	}

	viper.SetConfigName("config")
	viper.SetConfigType("yaml") // will support both yaml and json
	
	// Add config paths
	configHome, err := os.UserConfigDir()
	if err == nil {
		viper.AddConfigPath(filepath.Join(configHome, "wheel"))
	}
	viper.AddConfigPath(".")
	
	// Set defaults
	viper.SetDefault("database.path", "wheel.db")
	
	// Read config
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return err
		}
	}
	
	return nil
}