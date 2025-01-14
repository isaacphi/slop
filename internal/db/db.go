package db

import (
	"gorm.io/gorm"
	"gorm.io/driver/sqlite"
	"github.com/spf13/viper"
)

var DB *gorm.DB

func Initialize() error {
	var err error
	DB, err = gorm.Open(sqlite.Open(viper.GetString("database.path")), &gorm.Config{})
	if err != nil {
		return err
	}

	// Auto-migrate the schema
	err = DB.AutoMigrate(
		&Conversation{},
		&Message{},
	)
	return err
}

type Conversation struct {
	gorm.Model
	Title     string
	Messages  []Message
	ModelName string
}

type Message struct {
	gorm.Model
	ConversationID uint
	Role          string // user, assistant, system
	Content       string
}