package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

/*
Config System Design:
This configuration system implements a hierarchical config with the following precedence
(highest to lowest priority):

1. Environment variables (for secrets and crucial overrides)
2. Local project config (.wheel/*.wheel.{yaml,json})
3. Global user config ($XDG_CONFIG_HOME/wheel/*.wheel.{yaml,json})
4. Default values (from defaults.wheel.yaml)

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

// Config holds the configuration and internal viper instance
type Config struct {
	v  *viper.Viper
	mu sync.RWMutex
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
	// Add more env vars here as needed
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
		if strings.HasSuffix(name, ".wheel.yaml") ||
			strings.HasSuffix(name, ".wheel.json") {
			files = append(files, filepath.Join(dir, name))
		}
	}
	return files, nil
}

// New creates a new config instance
func New(verbose bool) (*Config, error) {
	c := &Config{
		v: viper.New(),
	}

	// Load defaults first
	if err := c.loadDefaults(); err != nil {
		return nil, fmt.Errorf("error loading defaults: %w", err)
	}

	// Set up env vars
	c.v.SetEnvPrefix("WHEEL")
	c.v.AutomaticEnv()
	for _, env := range envVars {
		c.v.BindEnv(env.key, env.envVar)
	}

	// Track sources for verbose logging
	sources := make(map[string][]configSource)

	// Load configs in order: global then local
	if err := c.loadConfigs(verbose, sources); err != nil {
		return nil, err
	}

	// Print verbose logging if enabled
	if verbose {
		c.logConfigSources(sources)
	}

	return c, nil
}

// loadDefaults loads the default configuration from the embedded defaults file
func (c *Config) loadDefaults() error {
	defaultsPath := filepath.Join("internal", "config", "defaults.wheel.yaml")
	c.v.SetConfigFile(defaultsPath)
	if err := c.v.ReadInConfig(); err != nil {
		return fmt.Errorf("could not read defaults file: %w", err)
	}
	return nil
}

func (c *Config) loadConfigs(verbose bool, sources map[string][]configSource) error {
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

	// Load local configs
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

			// Track sources if verbose logging is enabled
			if verbose {
				settings := v.AllSettings()
				c.trackSources(sources, settings, f)
			}

			// Merge with specific strategy
			if err := c.mergeConfig(v.AllSettings()); err != nil {
				return fmt.Errorf("error merging config from %s: %w", f, err)
			}
		}
	}

	// Add environment variable sources if verbose
	if verbose {
		for _, env := range envVars {
			if val := os.Getenv(env.envVar); val != "" {
				displayVal := val
				if env.isSecret {
					displayVal = "[REDACTED]"
				}
				sources[env.key] = append(sources[env.key], configSource{
					value:  displayVal,
					source: fmt.Sprintf("%s environment variable", env.envVar),
				})
			}
		}
	}

	return nil
}

type configSource struct {
	value  interface{}
	source string
}

func (c *Config) mergeConfig(settings map[string]interface{}) error {
	for key, value := range settings {
		existing := c.v.Get(key)
		if existing == nil {
			// Key doesn't exist, just set it
			c.v.Set(key, value)
			continue
		}

		// Handle different types
		switch existingVal := existing.(type) {
		case []interface{}:
			// For slices, append new values and remove duplicates
			if newSlice, ok := value.([]interface{}); ok {
				// Create a map to track unique values
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
			// For maps, recursively merge
			if newMap, ok := value.(map[string]interface{}); ok {
				merged := mergeMapRecursive(existingVal, newMap)
				c.v.Set(key, merged)
			} else {
				return fmt.Errorf("type mismatch for key %s: expected map, got %T", key, value)
			}

		default:
			// For all other types, override
			c.v.Set(key, value)
		}
	}
	return nil
}

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

func (c *Config) trackSources(sources map[string][]configSource, settings map[string]interface{}, filename string) {
	for key, value := range settings {
		sources[key] = append(sources[key], configSource{
			value:  value,
			source: filename,
		})
	}
}

// PrintConfig prints the final merged configuration
func (c *Config) PrintConfig() {
	fmt.Println("CONFIG:")
	c.mu.RLock()
	defer c.mu.RUnlock()

	settings := c.v.AllSettings()

	// Convert to JSON for consistent formatting
	jsonBytes, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		log.Printf("Error marshaling config: %v", err)
		return
	}

	// Convert back to interface{} for YAML marshal
	var out interface{}
	if err := json.Unmarshal(jsonBytes, &out); err != nil {
		log.Printf("Error unmarshaling config: %v", err)
		return
	}

	// Convert to YAML for better readability
	yamlBytes, err := yaml.Marshal(out)
	if err != nil {
		log.Printf("Error converting to YAML: %v", err)
		return
	}

	// Print final config, but redact sensitive values
	lines := strings.Split(string(yamlBytes), "\n")
	for _, line := range lines {
		if strings.Contains(strings.ToLower(line), "key") ||
			strings.Contains(strings.ToLower(line), "secret") ||
			strings.Contains(strings.ToLower(line), "password") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				fmt.Printf("%s: [REDACTED]\n", parts[0])
				continue
			}
		}
		fmt.Println(line)
	}
}

func (c *Config) logConfigSources(sources map[string][]configSource) {
	log.Println("Configuration values and their sources:")
	for key, sourceList := range sources {
		log.Printf("Key: %s", key)

		// For lists and maps, show all sources
		value := c.v.Get(key)
		switch value.(type) {
		case []interface{}, map[string]interface{}:
			// Show all sources for lists and maps as they contribute to final value
			for _, source := range sourceList {
				if strings.Contains(strings.ToLower(key), "key") ||
					strings.Contains(strings.ToLower(key), "secret") ||
					strings.Contains(strings.ToLower(key), "password") {
					log.Printf("  - [REDACTED] from %s", source.source)
				} else {
					log.Printf("  - %v from %s", source.value, source.source)
				}
			}
		default:
			// For scalar values, only show the final value
			if strings.Contains(strings.ToLower(key), "key") ||
				strings.Contains(strings.ToLower(key), "secret") ||
				strings.Contains(strings.ToLower(key), "password") {
				log.Printf("  - [REDACTED] from %s", sourceList[len(sourceList)-1].source)
			} else {
				log.Printf("  - %v from %s", c.v.Get(key), sourceList[len(sourceList)-1].source)
			}
		}
	}
}

// Validate validates the configuration against the schema
func (c *Config) Validate() error {
	var schema ConfigSchema
	if err := c.v.Unmarshal(&schema); err != nil {
		return fmt.Errorf("error unmarshaling config: %w", err)
	}

	validate := validator.New()
	if err := validate.Struct(schema); err != nil {
		return fmt.Errorf("config validation error: %w", err)
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
			return fmt.Errorf("mainModel %q must be one of configured models: %v", schema.MainModel, schema.Models)
		}
	}

	return nil
}

// Getter methods that delegate to the internal viper instance
func (c *Config) Get(key string) interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.v.Get(key)
}

func (c *Config) GetString(key string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.v.GetString(key)
}

func (c *Config) GetInt(key string) int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.v.GetInt(key)
}

func (c *Config) GetBool(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.v.GetBool(key)
}

func (c *Config) GetFloat64(key string) float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.v.GetFloat64(key)
}

func (c *Config) GetStringSlice(key string) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.v.GetStringSlice(key)
}

func (c *Config) GetStringMap(key string) map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.v.GetStringMap(key)
}

func (c *Config) GetStringMapString(key string) map[string]string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.v.GetStringMapString(key)
}

func (c *Config) IsSet(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.v.IsSet(key)
}

// Get all settings
func (c *Config) AllSettings() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.v.AllSettings()
}

// Get typed config
func (c *Config) GetConfig() (*ConfigSchema, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var cfg ConfigSchema
	if err := c.v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}
	return &cfg, nil
}
