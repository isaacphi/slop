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
2. Local project config (.wheel/*.wheel.{yaml,json})
3. Global user config ($XDG_CONFIG_HOME/wheel/*.wheel.{yaml,json})
4. Default values (from defaults.wheel.yaml)
5. JSON and YAML support

The system supports:
- Multiple config files in each directory, merged alphabetically
- Automatic merging of lists (they combine)
- Deep merging of maps
- Override of scalar values
- Verbose logging of where each config value originated
- Schema validation of the final config
- Easy addition of new environment variable mappings

Example:
If you have these files:
~/.config/wheel/models.wheel.yaml:  { models: ["gpt-4"] }
./.wheel/models.wheel.yaml:         { models: ["claude"] }
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

func New(verbose bool) (*ConfigSchema, error) {
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
	var unknown []string
	for _, key := range c.v.AllKeys() {
		if !known[key] {
			unknown = append(unknown, key)
		}
	}
	if len(unknown) > 0 {
		log.Printf("Warning: Found configuration values not in schema: %v", unknown)
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
	globalDir := filepath.Join(xdgConfig, "wheel")
	localDir := ".wheel"

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

// findConfigFiles returns all *.wheel.{yaml,json} files in a directory
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
		if strings.HasSuffix(name, "wheel.yaml") ||
			strings.HasSuffix(name, "wheel.json") {
			files = append(files, filepath.Join(dir, name))
		}
	}
	return files, nil
}

// validateConfig creates and validates a ConfigSchema
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
	if schema.MainModel != "" {
		found := false
		for _, model := range schema.Models {
			if model == schema.MainModel {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("mainModel %q must be one of configured models: %v",
				schema.MainModel, schema.Models)
		}
	}

	return &schema, nil
}

func (c *Config) mergeConfig(settings map[string]interface{}) error {
	for key, value := range settings {
		existing := c.v.Get(key)
		if existing == nil {
			c.v.Set(key, value)
			continue
		}

		switch existingVal := existing.(type) {
		case []interface{}:
			if newSlice, ok := value.([]interface{}); ok {
				seen := make(map[interface{}]bool)
				combined := make([]interface{}, 0)

				// Add existing values
				for _, v := range existingVal {
					if !seen[v] {
						seen[v] = true
						combined = append(combined, v)
					}
				}

				// Add new values
				for _, v := range newSlice {
					if !seen[v] {
						seen[v] = true
						combined = append(combined, v)
					}
				}

				c.v.Set(key, combined)
			} else {
				return fmt.Errorf("type mismatch for key %s: expected slice, got %T", key, value)
			}

		case map[string]interface{}:
			if newMap, ok := value.(map[string]interface{}); ok {
				merged := mergeMapRecursive(existingVal, newMap)
				c.v.Set(key, merged)
			} else {
				return fmt.Errorf("type mismatch for key %s: expected map, got %T", key, value)
			}

		default:
			c.v.Set(key, value)
		}
	}
	return nil
}

// mergeMapRecursive recursively merges two maps
func mergeMapRecursive(existing, new map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	// Copy existing map
	for k, v := range existing {
		result[k] = v
	}

	// Merge new map
	for k, v := range new {
		if existing[k] == nil {
			result[k] = v
			continue
		}

		switch existingVal := existing[k].(type) {
		case map[string]interface{}:
			if newVal, ok := v.(map[string]interface{}); ok {
				result[k] = mergeMapRecursive(existingVal, newVal)
			} else {
				result[k] = v
			}
		case []interface{}:
			if newVal, ok := v.([]interface{}); ok {
				result[k] = append(existingVal, newVal...)
			} else {
				result[k] = v
			}
		default:
			result[k] = v
		}
	}

	return result
}
