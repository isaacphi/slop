package config

import "github.com/invopop/jsonschema"

// GenerateJSONSchema generates a JSON schema for the configuration
func GenerateJSONSchema() (*jsonschema.Schema, error) {
	r := &jsonschema.Reflector{
		RequiredFromJSONSchemaTags: true,
		AllowAdditionalProperties:  false,
		DoNotReference:             false,
	}

	schema := r.Reflect(&ConfigSchema{})

	schema.Title = "Slop Configuration Schema"
	schema.Description = "Configuration schema for the Slop CLI tool"

	return schema, nil
}
