//go:generate go run ../../cmd/tools/genschema/main.go
package config

// `mapstructure` tags are used by viper when unmarshalling yaml or json
// `json` and `jsonschema` tags are used to generate schema.json and for default values
// note that default values in default.slop.yaml take precedence

// ConfigSchema is the root configuration object
type ConfigSchema struct {
	Presets       map[string]Preset    `mapstructure:"presets" json:"presets" jsonschema:"description=Available model configurations"`
	DefaultPreset string               `mapstructure:"defaultPreset" json:"defaultPreset" jsonschema:"description=Default preset for new chats,default=claude"`
	DBPath        string               `mapstructure:"dbPath" json:"dbPath" jsonschema:"description=Path to the database file,default=.slop/slop.db"`
	Internal      Internal             `mapstructure:"internal" json:"internal" jsonschema:"description=Internal configuration settings"`
	MCPServers    map[string]MCPServer `mapstructure:"mcpServers" json:"mcpServers" jsonschema:"description=MCP server configurations"`
	Log           Log                  `mapstructure:"log" json:"log" jsonschema:"description=Logging configuration"`
	Toolsets      map[string]Toolset   `mapstructure:"toolsets" json:"toolsets" jsonschema:"description=Configurations for sets of MCP Servers and tools. Leave empty to allow all servers and all tools."`
	Prompts       map[string]Prompt    `mapstructure:"prompts" json:"prompts" jsonschema:"Reusable prompt configuration"`
	KeyMap        KeyMap               `mapstructure:"keyMap" json:"keyMap" jsonschema:"description=Custom keybindings for the TUI"`

	// Internal fields for printing
	sources  map[string]string
	warnings []string
}

// LLM presets
type Preset struct {
	Provider       string   `mapstructure:"provider" json:"provider" jsonschema:"description=The AI provider to use"`
	Name           string   `mapstructure:"name" json:"name" jsonschema:"description=Model name for the provider"`
	MaxTokens      int      `mapstructure:"maxTokens" json:"maxTokens" jsonschema:"description=Maximum tokens to use in requests,default=1000"`
	Temperature    float64  `mapstructure:"temperature" json:"temperature" jsonschema:"description=Temperature setting for the model,default=0.7"`
	Toolsets       []string `mapstructure:"toolsets" json:"toolsets" jsonschema:"description=Toolsets to use for this model preset"`
	SystemMessage  string   `mapstructure:"systemMessage" json:"systemMessage" jsonschema:"description=Base system message for all conversations using this preset"`
	IncludePrompts []string `mapstructure:"includePrompts" json:"includePrompts" jsonschema:"description=Names of prompts to include in the system message,default=false"`
}

// Prompts
type Prompt struct {
	Content                string `mapstructure:"content" json:"content" jsonschema:"description=The text content of the prompt"`
	IncludeInSystemMessage bool   `mapstructure:"includeInSystemMessage" json:"includeInSystemMessage" jsonschema:"description=If true, this prompt will be automatically included in all system messages"`
	SystemMessageTrigger   string `mapstructure:"systemMessageTrigger" json:"systemMessageTrigger" jsonschema:"description=Regex pattern - if matched in user message or history, this prompt will be included in the system message"`
}

// Toolsets
type Toolset struct {
	Servers       map[string]MCPServerToolConfig `mapstructure:"servers" json:"servers"`
	SystemMessage string                         `mapstructure:"systemMessage" json:"systemMessage" jsonschema:"description=System message to include when this toolset is used"`
}

type MCPServerToolConfig struct {
	RequireApproval bool                  `mapstructure:"requireApproval" json:"requireApproval" jsonschema:"description=Whether tools need explicit approval,default=true"`
	AllowedTools    map[string]ToolConfig `mapstructure:"allowedTools" json:"allowedTools" jsonschema:"description=Configuration for allowed tools. Leave empty to allow all tools."`
}

type ToolConfig struct {
	RequireApproval  bool              `mapstructure:"requireApproval" json:"requireApproval" jsonschema:"description=Whether tools need explicit approval,default=true"`
	PresetParameters map[string]string `mapstructure:"presetParameters" json:"presetParameters" jsonschema:"description=Pre-configured parameters for this tool. Uses partial function application to send fewer parameters to the LLM."`
}

// Internal configuration settings
type Internal struct {
	Model         string `mapstructure:"model" json:"model" jsonschema:"description=Default model to use for internal llm calls such as summaries,default=claude"`
	SummaryPrompt string `mapstructure:"summaryPrompt" json:"summaryPrompt" jsonschema:"description=Prompt used for generating conversation summaries"`
}

// MCP server configuration
type MCPServer struct {
	Command       string            `mapstructure:"command" json:"command" jsonschema:"description=Command to run the MCP server"`
	Args          []string          `mapstructure:"args" json:"args" jsonschema:"description=Command line arguments for the MCP server"`
	Env           map[string]string `mapstructure:"env" json:"env" jsonschema:"description=Environment variables for the MCP server"`
	SystemMessage string            `mapstructure:"systemMessage" json:"systemMessage" jsonschema:"description=System message to include when any of this server's tools are used"`
}

// Logging configuration
type Log struct {
	LogLevel string `mapstructure:"logLevel" json:"logLevel" jsonschema:"description=Log level (DEBUG, INFO, WARN, ERROR),default=INFO,enum=DEBUG,enum=INFO,enum=WARN,enum=ERROR"`
	LogFile  string `mapstructure:"logFile" json:"logFile" jsonschema:"description=Log file path, empty for stdout,default="`
}
