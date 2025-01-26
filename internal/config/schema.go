package config

type Model struct {
	Provider    string  `mapstructure:"provider"`
	Name        string  `mapstructure:"name"`
	MaxLength   int     `mapstructure:"maxLength"`
	Temperature float64 `mapstructure:"temperature"`
}

type ConfigSchema struct {
	Models      map[string]Model             `mapstructure:"models"`
	ActiveModel string                       `mapstructure:"activeModel"`
	Theme       map[string]map[string]string `mapstructure:"theme"`
	DBPath      string                       `mapstructure:"dbPath"`

	// Internal fields for printing
	sources  map[string][]configSource
	defaults map[string]interface{}
}
