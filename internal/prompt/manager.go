package prompt

type Template struct {
	Name        string
	Description string
	Template    string
	Variables   []string
}

type Manager struct {
	templates map[string]*Template
}

func NewManager() *Manager {
	return &Manager{
		templates: make(map[string]*Template),
	}
}

func (m *Manager) LoadTemplate(name string) (*Template, error) {
	if template, ok := m.templates[name]; ok {
		return template, nil
	}
	return nil, nil
}

func (m *Manager) RenderTemplate(template *Template, variables map[string]string) (string, error) {
	// TODO: Implement template rendering with variables
	return template.Template, nil
}