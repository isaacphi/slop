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
	Summary  string    `gorm:"type:text"`
	Messages []Message `gorm:"foreignKey:ThreadID"`
	gorm.Model
}

type Message struct {
	ID       uuid.UUID `gorm:"type:uuid;primary_key"`
	ThreadID uuid.UUID `gorm:"type:uuid;index"`
	Thread   *Thread   `gorm:"foreignKey:ThreadID"`

	ParentID *uuid.UUID `gorm:"type:uuid;index"`
	Parent   *Message   `gorm:"foreignKey:ParentID"`
	Children []Message  `gorm:"foreignKey:ParentID"`

	Role      Role   `gorm:"type:text"`
	Content   string `gorm:"type:text"`
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
