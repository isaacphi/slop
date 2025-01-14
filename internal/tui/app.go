package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/isaacphi/wheel/internal/db"
	"github.com/isaacphi/wheel/internal/llm"
	"github.com/isaacphi/wheel/internal/tui/components"
	"github.com/spf13/viper"
)

type AppState int

const (
	StateConversationList AppState = iota
	StateChat
)

type Model struct {
	state          AppState
	conversationCh chan db.Conversation
	client         *llm.Client
	currentModel   tea.Model
}

func NewModel() (Model, error) {
	client, err := llm.NewClient("openai", viper.GetString("openai.api_key"))
	if err != nil {
		return Model{}, err
	}

	ch := make(chan db.Conversation, 1)
	list := components.NewConversationList(ch)

	return Model{
		state:          StateConversationList,
		conversationCh: ch,
		client:         client,
		currentModel:   list,
	}, nil
}

func (m Model) Init() tea.Cmd {
	return m.currentModel.Init()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}
	}

	// Handle conversation selection
	select {
	case conv := <-m.conversationCh:
		if conv.ID == 0 {
			// Create new conversation
			conv = db.Conversation{
				Title: "New Conversation",
			}
			db.DB.Create(&conv)
		}
		m.state = StateChat
		m.currentModel = components.NewChatView(conv, m.client)
		return m, m.currentModel.Init()
	default:
	}

	// Update current model
	var newModel tea.Model
	newModel, cmd = m.currentModel.Update(msg)
	m.currentModel = newModel
	return m, cmd
}

func (m Model) View() string {
	return m.currentModel.View()
}