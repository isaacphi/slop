//go:generate go run ../../cmd/tools/genschema/main.go
package config

// `mapstructure` tags are used by viper when unmarshalling yaml or json
// `json` and `jsonschema` tags are used to generate schema.json and for default values
// note that default values in default.slop.yaml take precedence

// LLM presets
type ModelPreset struct {
	Provider    string   `mapstructure:"provider" json:"provider" jsonschema:"description=The AI provider to use"`
	Name        string   `mapstructure:"name" json:"name" jsonschema:"description=Model name for the provider"`
	MaxTokens   int      `mapstructure:"maxTokens" json:"maxTokens" jsonschema:"description=Maximum tokens to use in requests,default=1000"`
	Temperature float64  `mapstructure:"temperature" json:"temperature" jsonschema:"description=Temperature setting for the model,default=0.7"`
	Toolsets    []string `mapstructure:"toolsets" json:"toolsets" jsonschema:"description=Toolsets to use for this model preset"`
}

type Parameters struct {
	Type       string              `mapstructure:"type" json:"type" jsonschema:"enum=object,default=object"`
	Properties map[string]Property `mapstructure:"properties" json:"properties" jsonschema:"description=Properties of the parameter object"`
	Required   []string            `mapstructure:"required" json:"required" jsonschema:"description=List of required property names"`
}

type Property struct {
	Type        string              `mapstructure:"type" json:"type" jsonschema:"description=JSON Schema type of the property"`
	Description string              `mapstructure:"description" json:"description" jsonschema:"description=Description of what the property does"`
	Enum        []string            `mapstructure:"enum,omitempty" json:"enum,omitempty" jsonschema:"description=Allowed values for this property"`
	Items       *Property           `mapstructure:"items,omitempty" json:"items,omitempty" jsonschema:"description=Schema for array items"`
	Properties  map[string]Property `mapstructure:"properties,omitempty" json:"properties,omitempty" jsonschema:"description=Nested properties for object types"`
	Required    []string            `mapstructure:"required,omitempty" json:"required,omitempty" jsonschema:"description=Required nested properties"`
	Default     interface{}         `mapstructure:"default,omitempty" json:"default,omitempty" jsonschema:"description=Default value for this property"`
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
	Command string            `mapstructure:"command" json:"command" jsonschema:"description=Command to run the MCP server"`
	Args    []string          `mapstructure:"args" json:"args" jsonschema:"description=Command line arguments for the MCP server"`
	Env     map[string]string `mapstructure:"env" json:"env" jsonschema:"description=Environment variables for the MCP server"`
}

// Agent configuration
type Agent struct {
	AutoApproveFunctions bool `mapstructure:"autoApproveFunctions" json:"autoApproveFunctions" jsonschema:"description=Automatically approve function calls,default=true"`
}

// Logging configuration
type Log struct {
	LogLevel string `mapstructure:"logLevel" json:"logLevel" jsonschema:"description=Log level (DEBUG, INFO, WARN, ERROR),default=INFO,enum=DEBUG,enum=INFO,enum=WARN,enum=ERROR"`
	LogFile  string `mapstructure:"logFile" json:"logFile" jsonschema:"description=Log file path, empty for stdout,default="`
}

// ConfigSchema is the root configuration object
type ConfigSchema struct {
	ModelPresets map[string]ModelPreset                    `mapstructure:"modelPresets" json:"modelPresets" jsonschema:"description=Available model configurations"`
	ActiveModel  string                                    `mapstructure:"activeModel" json:"activeModel" jsonschema:"description=Currently selected model preset,default=claude"`
	DBPath       string                                    `mapstructure:"dbPath" json:"dbPath" jsonschema:"description=Path to the database file,default=.slop/slop.db"`
	Internal     Internal                                  `mapstructure:"internal" json:"internal" jsonschema:"description=Internal configuration settings"`
	MCPServers   map[string]MCPServer                      `mapstructure:"mcpServers" json:"mcpServers" jsonschema:"description=MCP server configurations"`
	Agent        Agent                                     `mapstructure:"agent" json:"agent" jsonschema:"description=Agent behavior settings"`
	Log          Log                                       `mapstructure:"log" json:"log" jsonschema:"description=Logging configuration"`
	Toolsets     map[string]map[string]MCPServerToolConfig `mapstructure:"toolsets" json:"toolsets" jsonschema:"description=Configurations for sets of MCP Servers and tools. Leave empty to allow all servers and all tools."`

	// Internal fields for printing
	sources  map[string]string
	warnings []string
}
