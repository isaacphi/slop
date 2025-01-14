package components

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/isaacphi/wheel/internal/db"
	"github.com/isaacphi/wheel/internal/tui/styles"
)

type ConversationItem struct {
	conversation db.Conversation
	title       string
}

func (i ConversationItem) Title() string       { return i.title }
func (i ConversationItem) Description() string { return "" }
func (i ConversationItem) FilterValue() string { return i.title }

type ConversationList struct {
	list     list.Model
	selected chan db.Conversation
}

func NewConversationList(selected chan db.Conversation) *ConversationList {
	l := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Conversations"
	l.Styles.Title = styles.ListTitleStyle
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)

	return &ConversationList{
		list:     l,
		selected: selected,
	}
}

func (m *ConversationList) Init() tea.Cmd {
	return loadConversations
}

func loadConversations() tea.Msg {
	var conversations []db.Conversation
	db.DB.Order("created_at desc").Find(&conversations)

	items := make([]list.Item, len(conversations))
	for i, conv := range conversations {
		items[i] = ConversationItem{
			conversation: conv,
			title:       conv.Title,
		}
	}
	return conversationsLoadedMsg{items}
}

type conversationsLoadedMsg struct {
	items []list.Item
}

func (m *ConversationList) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		m.list.SetHeight(msg.Height)
		return m, nil

	case tea.KeyMsg:
		// Handle key presses before passing to list
		switch msg.String() {
		case "n":
			// Create an empty conversation to signal new conversation creation
			m.selected <- db.Conversation{}
			return m, nil
		case "enter":
			if i, ok := m.list.SelectedItem().(ConversationItem); ok {
				m.selected <- i.conversation
				return m, nil
			}
		case "q", "ctrl+c":
			return m, tea.Quit
		}

	case tea.MouseMsg:
		if msg.Type == tea.MouseLeft {
			if i, ok := m.list.SelectedItem().(ConversationItem); ok {
				m.selected <- i.conversation
				return m, nil
			}
		}

	case conversationsLoadedMsg:
		m.list.SetItems(msg.items)
	}

	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m ConversationList) View() string {
	return styles.DocStyle.Render(
		m.list.View() + "\n\nPress 'n' to start a new conversation",
	)
}