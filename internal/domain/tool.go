package domain

type Tool struct {
	Name        string
	Description string
	Parameters  Parameters
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
