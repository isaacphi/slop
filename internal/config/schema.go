package config

type Model struct {
	Provider    string  `mapstructure:"provider"`
	Name        string  `mapstructure:"name"`
	MaxTokens   int     `mapstructure:"MaxTokens"`
	Temperature float64 `mapstructure:"temperature"`
}

type ConfigSchema struct {
	Models      map[string]Model `mapstructure:"models"`
	ActiveModel string           `mapstructure:"activeModel"`
	DBPath      string           `mapstructure:"dbPath"`

	// Internal fields for printing
	sources  map[string][]configSource
	defaults map[string]interface{}
}
