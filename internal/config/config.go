package config

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
)

/*
Config System Design:
This configuration system implements a hierarchical config with the following precedence
(highest to lowest priority):

1. Environment variables for secrets
2. Local project config (.slop/*.slop.{yaml,json})
3. Global user config ($XDG_CONFIG_HOME/slop/*.slop.{yaml,json})
4. Default values (from defaults.slop.yaml)
5. JSON and YAML support

The system supports:
- Multiple config files in each directory, merged alphabetically
- Automatic merging of lists (they combine)
- Deep merging of maps
- Override of scalar values
- Schema validation of the final config
- Easy addition of new environment variable mappings

Example:
If you have these files:
~/.config/slop/models.slop.yaml:  { models: ["gpt-4"] }
./.slop/models.slop.yaml:         { models: ["claude"] }
The result will be: { models: ["gpt-4", "claude"] }
*/

// Config holds the configuration state
type Config struct {
	v       *viper.Viper
	mu      sync.RWMutex
	sources map[string][]configSource
}

// configSource tracks where each value came from
type configSource struct {
	value  interface{}
	source string
}

// envVarConfig defines an environment variable mapping
type envVarConfig struct {
	key      string // Key in the config
	envVar   string // Environment variable name
	isSecret bool   // Whether to redact in logs
}

// Environment variables to load
var envVars = []envVarConfig{
	{key: "llm_key", envVar: "OPENAI_API_KEY", isSecret: true},
	{key: "llm_key", envVar: "ANTHROPIC_API_KEY", isSecret: true},
}

func New() (*ConfigSchema, error) {
	c := &Config{
		v:       viper.New(),
		sources: make(map[string][]configSource),
	}

	// Load defaults first
	if err := c.loadDefaults(); err != nil {
		return nil, fmt.Errorf("error loading defaults: %w", err)
	}

	// Load configs and environment variables
	if err := c.loadConfigs(); err != nil {
		return nil, err
	}

	// Check for unknown keys using schema
	known := GetKnownKeys()
	for _, key := range c.v.AllKeys() {
		if !IsKnownKey(known, key) {
			log.Printf("Warning: Found configuration value not in schema: %v", key)
		}
	}

	// Validate and create type-safe config
	schema, err := c.validateConfig()
	if err != nil {
		return nil, fmt.Errorf("config validation error: %w", err)
	}

	// Add sources and defaults to schema for printing
	schema.sources = c.sources
	schema.defaults = getDefaultsMap()

	return schema, nil
}

// getDefaultsMap returns the default values from the embedded config
func getDefaultsMap() map[string]interface{} {
	v := viper.New()
	v.SetConfigType("yaml")
	if err := v.ReadConfig(bytes.NewBuffer(defaultConfig)); err != nil {
		return nil
	}
	return v.AllSettings()
}

// loadDefaults loads the embedded default configuration
func (c *Config) loadDefaults() error {
	v := viper.New()
	v.SetConfigType("yaml")
	if err := v.ReadConfig(bytes.NewBuffer(defaultConfig)); err != nil {
		return fmt.Errorf("could not read defaults: %w", err)
	}

	// Track default sources
	settings := v.AllSettings()
	for key, value := range settings {
		c.sources[key] = []configSource{{
			value:  value,
			source: "default",
		}}
	}

	// Merge defaults into main config
	if err := c.mergeConfig(settings); err != nil {
		return fmt.Errorf("could not merge defaults: %w", err)
	}

	return nil
}

func (c *Config) loadConfigs() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Find global config directory
	xdgConfig := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfig == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		xdgConfig = filepath.Join(home, ".config")
	}
	globalDir := filepath.Join(xdgConfig, "slop")
	localDir := ".slop"

	// Load files from both locations
	for _, dir := range []string{globalDir, localDir} {
		files, err := findConfigFiles(dir)
		if err != nil && !os.IsNotExist(err) {
			return err
		}

		for _, f := range files {
			v := viper.New()
			v.SetConfigFile(f)
			if err := v.ReadInConfig(); err != nil {
				return fmt.Errorf("error reading config file %s: %w", f, err)
			}

			settings := v.AllSettings()

			// Track sources of values
			for key, value := range settings {
				c.sources[key] = append(c.sources[key], configSource{
					value:  value,
					source: f,
				})
			}

			// Merge with existing config
			if err := c.mergeConfig(settings); err != nil {
				return fmt.Errorf("error merging config from %s: %w", f, err)
			}
		}
	}

	// Add environment variable sources
	for _, env := range envVars {
		if val := os.Getenv(env.envVar); val != "" {
			displayVal := val
			if env.isSecret {
				displayVal = "[REDACTED]"
			}
			c.sources[env.key] = append(c.sources[env.key], configSource{
				value:  displayVal,
				source: fmt.Sprintf("%s environment variable", env.envVar),
			})
		}
	}

	return nil
}

// findConfigFiles returns all *.slop.{yaml,json} files in a directory
func findConfigFiles(dir string) ([]string, error) {
	var files []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, "slop.yaml") ||
			strings.HasSuffix(name, "slop.json") {
			files = append(files, filepath.Join(dir, name))
		}
	}
	return files, nil
}

func (c *Config) validateConfig() (*ConfigSchema, error) {
	var schema ConfigSchema
	if err := c.v.Unmarshal(&schema); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	validate := validator.New()
	if err := validate.Struct(schema); err != nil {
		return nil, fmt.Errorf("config validation error: %w", err)
	}

	// Additional custom validations
	if schema.ActiveModel != "" {
		found := false
		for key := range schema.Models {
			if key == schema.ActiveModel {
				found = true
				break
			}
		}
		if !found {
			var availableModels []string
			for key := range schema.Models {
				availableModels = append(availableModels, key)
			}
			return nil, fmt.Errorf("mainModel %q must be one of configured models: %v",
				schema.ActiveModel, availableModels)
		}
	}

	return &schema, nil
}

func (c *Config) mergeConfig(settings map[string]interface{}) error {
	// Convert settings to flat map with dot notation
	flat := flattenMap(settings, "")

	// Set each value
	for key, value := range flat {
		c.v.Set(key, value)
	}
	return nil
}

// flattenMap converts a nested map to a flat map with dot notation
func flattenMap(m map[string]interface{}, prefix string) map[string]interface{} {
	result := make(map[string]interface{})

	for k, v := range m {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}

		switch val := v.(type) {
		case map[string]interface{}:
			// Recursively flatten nested maps
			flattened := flattenMap(val, key)
			for fk, fv := range flattened {
				result[fk] = fv
			}
		case map[interface{}]interface{}:
			// Convert to map[string]interface{} and recurse
			stringMap := make(map[string]interface{})
			for mk, mv := range val {
				if skeyStr, ok := mk.(string); ok {
					stringMap[skeyStr] = mv
				}
			}
			flattened := flattenMap(stringMap, key)
			for fk, fv := range flattened {
				result[fk] = fv
			}
		default:
			result[key] = v
		}
	}

	return result
}
