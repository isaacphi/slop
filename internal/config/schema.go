package config

// LLM presets
type Model struct {
	Provider    string  `mapstructure:"provider"`
	Name        string  `mapstructure:"name"`
	MaxTokens   int     `mapstructure:"MaxTokens"`
	Temperature float64 `mapstructure:"temperature"`
}

// Internal configuration settings
type Internal struct {
	Model         string `mapstructure:"model"`
	SummaryPrompt string `mapstructure:"summaryPrompt"`
}

type ConfigSchema struct {
	Models      map[string]Model `mapstructure:"models"`
	ActiveModel string           `mapstructure:"activeModel"`
	DBPath      string           `mapstructure:"dbPath"`
	Internal    Internal         `mapstructure:"internal"`

	// Internal fields for printing
	sources  map[string][]configSource
	defaults map[string]interface{}
}
