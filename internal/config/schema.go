package config

type ConfigSchema struct {
	Models      []string `mapstructure:"models"`
	MainModel   string   `mapstructure:"mainModel"`
	Prompts     []string `mapstructure:"prompts"`
	MaxTokens   int      `mapstructure:"maxTokens"`
	Temperature float64  `mapstructure:"temperature"`
	LLMKey      string   `mapstructure:"llm_key"`
}
