package config

type Model struct {
	Provider string `mapstructure:"provider"`
	Name     string `mapstructure:"name"`
}

type ConfigSchema struct {
	Models      map[string]Model             `mapstructure:"models"`
	MainModel   string                       `mapstructure:"mainModel"`
	Theme       map[string]map[string]string `mapstructure:"theme"`
	MaxTokens   int                          `mapstructure:"maxTokens"`
	Temperature float64                      `mapstructure:"temperature"`
	LLMKey      string                       `mapstructure:"llm_key"`
	DBPath      string                       `mapstructure:"DBPath"`

	// Internal fields for printing
	sources  map[string][]configSource
	defaults map[string]interface{}
}

