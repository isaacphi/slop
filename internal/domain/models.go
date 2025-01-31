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

type Thread struct {
	ID       uuid.UUID `gorm:"type:uuid;primary_key"`
	Messages []Message
	gorm.Model
}

type Message struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key"`
	ThreadID  uuid.UUID `gorm:"type:uuid"`
	Role      Role      `gorm:"type:text"`
	Content   string
	ModelName string `gorm:"type:text"`
	Provider  string `gorm:"type:text"`
	gorm.Model
}

func (t *Thread) BeforeCreate(tx *gorm.DB) (err error) {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	return
}

func (m *Message) BeforeCreate(tx *gorm.DB) (err error) {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return
}
