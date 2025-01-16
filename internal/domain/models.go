package domain

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Role string

const (
	RoleHuman     Role = "human"
	RoleAssistant Role = "assistant"
)

type Conversation struct {
	ID       uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primary_key"`
	Messages []Message
	gorm.Model
}

type Message struct {
	ID             uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primary_key"`
	ConversationID uuid.UUID `gorm:"type:uuid"`
	Role           Role      `gorm:"type:text"`
	Content        string
	gorm.Model
}
