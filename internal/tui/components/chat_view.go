package components

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/isaacphi/wheel/internal/db"
	"github.com/isaacphi/wheel/internal/llm"
	"github.com/isaacphi/wheel/internal/tui/styles"
)

type ChatView struct {
	conversation db.Conversation
	viewport     viewport.Model
	textarea     textarea.Model
	messages     []db.Message
	client       *llm.Client
	err          error
	ready        bool
	streaming    bool
}

func NewChatView(conversation db.Conversation, client *llm.Client) *ChatView {
	ta := textarea.New()
	ta.Placeholder = "Send a message..."
	ta.Focus()

	ta.Prompt = "┃ "
	ta.CharLimit = 280

	ta.SetWidth(30)
	ta.SetHeight(3)

	// Remove cursor line styling
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()

	vp := viewport.New(30, 30)
	vp.SetContent("")

	return &ChatView{
		conversation: conversation,
		textarea:     ta,
		viewport:     vp,
		client:       client,
		messages:     make([]db.Message, 0),
	}
}

func (m *ChatView) Init() tea.Cmd {
	return tea.Batch(
		loadMessages(m.conversation.ID),
		textarea.Blink,
	)
}

type messagesLoadedMsg struct {
	messages []db.Message
}

func loadMessages(conversationID uint) tea.Cmd {
	return func() tea.Msg {
		var messages []db.Message
		db.DB.Where("conversation_id = ?", conversationID).Order("created_at asc").Find(&messages)
		return messagesLoadedMsg{messages}
	}
}

type streamMsg struct {
	content string
}

func (m *ChatView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-m.textarea.Height()-4)
			m.textarea.SetWidth(msg.Width)
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - m.textarea.Height() - 4
			m.textarea.SetWidth(msg.Width)
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			if !m.textarea.Focused() {
				m.textarea.Focus()
				return m, nil
			}
			if m.streaming {
				return m, nil
			}
			content := m.textarea.Value()
			if content == "" {
				return m, nil
			}
			m.textarea.Reset()
			return m, m.sendMessage(content)
		}

	case messagesLoadedMsg:
		m.messages = msg.messages
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()

	case streamMsg:
		m.streaming = false
		m.messages = append(m.messages, db.Message{
			ConversationID: m.conversation.ID,
			Role:           "assistant",
			Content:        msg.content,
		})
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()
	}

	if !m.streaming {
		m.textarea, tiCmd = m.textarea.Update(msg)
	}
	m.viewport, vpCmd = m.viewport.Update(msg)

	return m, tea.Batch(tiCmd, vpCmd)
}

func (m *ChatView) View() string {
	return fmt.Sprintf(
		"%s\n%s",
		m.viewport.View(),
		m.textarea.View(),
	)
}

func (m *ChatView) renderMessages() string {
	var b strings.Builder
	for _, msg := range m.messages {
		if msg.Role == "user" {
			b.WriteString(styles.HighlightStyle.Render("You: "))
		} else {
			b.WriteString(styles.HighlightStyle.Render("Assistant: "))
		}
		b.WriteString(msg.Content)
		b.WriteString("\n\n")
	}
	return b.String()
}

func (m *ChatView) sendMessage(content string) tea.Cmd {
	return func() tea.Msg {
		userMsg := db.Message{
			ConversationID: m.conversation.ID,
			Role:           "user",
			Content:        content,
		}
		db.DB.Create(&userMsg)

		m.messages = append(m.messages, userMsg)
		m.streaming = true

		response, err := m.client.SendMessage(context.Background(), content)
		if err != nil {
			m.err = err
			return nil
		}

		assistantMsg := db.Message{
			ConversationID: m.conversation.ID,
			Role:           "assistant",
			Content:        response,
		}
		db.DB.Create(&assistantMsg)

		return streamMsg{content: response}
	}
}

