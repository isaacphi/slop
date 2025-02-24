package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
)

/*
Config System Design:
This configuration system implements a hierarchical config with the following precedence
(highest to lowest priority):

1. Command line overrides
2. Local project config (.slop/*.slop.{yaml,json})
3. Global user config ($XDG_CONFIG_HOME/slop/*.slop.{yaml,json})
4. Default values (from defaults.slop.yaml)

The system supports:
- Multiple config files in each directory, merged alphabetically
- Automatic merging of lists (they combine)
- Deep merging of maps
- Override of scalar values
- Schema validation of the final config

Example:
If you have these files:
~/.config/slop/models.slop.yaml:  { models: ["gpt-4"] }
./.slop/models.slop.yaml:         { models: ["claude"] }
The result will be: { models: ["gpt-4", "claude"] }
*/

// Config holds the configuration state
type Config struct {
	v        *viper.Viper
	mu       sync.RWMutex
	sources  map[string]string
	warnings []string
}

// RuntimeOverrides holds configuration values that can be overridden at runtime
// via CLI flags or other means
type RuntimeOverrides struct {
	LogLevel *string
	LogFile  *string
}

// Instantiate a new ConfigSchema
func New(overrides *RuntimeOverrides) (*ConfigSchema, error) {
	c := &Config{
		v:        viper.New(),
		sources:  make(map[string]string),
		warnings: make([]string, 0),
	}

	// Load defaults first
	if err := c.loadDefaults(); err != nil {
		return nil, fmt.Errorf("error loading defaults: %w", err)
	}

	// Load configs
	if err := c.loadConfigs(); err != nil {
		return nil, err
	}

	// Validate and create type-safe config
	schema, err := c.validateConfig()
	if err != nil {
		return nil, fmt.Errorf("config validation error: %w", err)
	}

	// Apply overrides
	if overrides != nil {
		if overrides.LogFile != nil {
			schema.Log.LogFile = *overrides.LogFile
			c.sources["log.logFile"] = "override"
		}
		if overrides.LogLevel != nil {
			schema.Log.LogLevel = *overrides.LogLevel
			c.sources["log.logLevel"] = "override"
		}
	}

	// Add sources to schema for printing
	schema.sources = c.sources
	schema.warnings = c.warnings

	return schema, nil
}

// loadDefaults loads the embedded default configuration
func (c *Config) loadDefaults() error {
	v := viper.New()
	v.SetConfigType("yaml")
	if err := v.ReadConfig(bytes.NewBuffer(defaultConfig)); err != nil {
		return fmt.Errorf("could not read defaults: %w", err)
	}

	settings := v.AllSettings()
	if err := c.mergeConfig(settings, "default"); err != nil {
		return fmt.Errorf("could not merge defaults: %w", err)
	}

	return nil
}

// Load all configuration files
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
			if err := c.mergeConfig(settings, f); err != nil {
				return fmt.Errorf("error merging config from %s: %w", f, err)
			}
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

// Validate the config against the schema and custom rules
func (c *Config) validateConfig() (*ConfigSchema, error) {
	var schema ConfigSchema
	if err := c.v.Unmarshal(&schema); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	// Set defaults from tags
	if err := setStructuralDefaults(&schema); err != nil {
		return nil, fmt.Errorf("error setting defaults: %w", err)
	}

	validate := validator.New()
	if err := validate.Struct(schema); err != nil {
		return nil, fmt.Errorf("config validation error: %w", err)
	}

	// Additional custom validations
	// TODO: also validate internal model
	if schema.DefaultPreset != "" {
		found := false
		for key := range schema.Presets {
			if key == schema.DefaultPreset {
				found = true
				break
			}
		}
		if !found {
			var availableModels []string
			for key := range schema.Presets {
				availableModels = append(availableModels, key)
			}
			return nil, fmt.Errorf("defaultPreset %q must be one of configured models: %v",
				schema.DefaultPreset, availableModels)
		}
	}
	// TODO: validate toolsets

	return &schema, nil
}

// Merge settings into the main Config.v viper instance
func (c *Config) mergeConfig(settings map[string]interface{}, source string) error {
	// Combine flattening and source tracking in one pass
	flat := c.flattenAndTrack(settings, "", source)

	// Set each value in Viper
	for key, value := range flat {
		c.v.Set(key, value)
	}
	return nil
}

// Build a map of js style dot notation settings to their source
func (c *Config) flattenAndTrack(m map[string]interface{}, prefix string, source string) map[string]interface{} {
	result := make(map[string]interface{})

	for k, v := range m {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}

		// Track the source
		c.sources[key] = source

		switch val := v.(type) {
		case map[string]interface{}:
			// Recursively flatten nested maps
			flattened := c.flattenAndTrack(val, key, source)
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
			flattened := c.flattenAndTrack(stringMap, key, source)
			for fk, fv := range flattened {
				result[fk] = fv
			}
		default:
			result[key] = v
		}
	}

	return result
}

func setStructuralDefaults(schema *ConfigSchema) error {
	return setDefaultsFromTags(reflect.ValueOf(schema).Elem())
}

func setDefaultsFromTags(v reflect.Value) error {
	if v.Kind() != reflect.Struct {
		return nil
	}

	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := v.Field(i)
		if !field.CanSet() {
			continue
		}

		tag := t.Field(i).Tag.Get("jsonschema")
		defaultVal := extractDefaultFromTag(tag)

		switch field.Kind() {
		case reflect.Struct:
			if err := setDefaultsFromTags(field); err != nil {
				return fmt.Errorf("field %s: %w", t.Field(i).Name, err)
			}

		case reflect.Map:
			if field.IsNil() {
				field.Set(reflect.MakeMap(field.Type()))
			}
			// If map values are structs, recursively set their defaults
			if field.Type().Elem().Kind() == reflect.Struct {
				// Create a new map to store updated values
				iter := field.MapRange()
				for iter.Next() {
					key := iter.Key()
					val := iter.Value()

					// Create a new value so we can modify it
					newVal := reflect.New(val.Type()).Elem()
					newVal.Set(val)

					// Recursively set defaults on the new value
					err := setDefaultsFromTags(newVal)
					if err != nil {
						return err
					}

					// Store the modified value back in the map
					field.SetMapIndex(key, newVal)
				}
			}

		case reflect.String:
			if field.String() == "" && defaultVal != "" {
				field.SetString(defaultVal)
			}

		case reflect.Bool:
			if defaultVal != "" {
				switch defaultVal {
				case "true":
					field.SetBool(true)
				case "false":
					field.SetBool(false)
				default:
					return fmt.Errorf("field %s: invalid boolean default value: %s (must be 'true' or 'false')",
						t.Field(i).Name, defaultVal)
				}
			}

		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if field.Int() == 0 && defaultVal != "" {
				val, err := strconv.ParseInt(defaultVal, 10, 64)
				if err != nil {
					return fmt.Errorf("field %s: invalid integer default value: %s (%w)",
						t.Field(i).Name, defaultVal, err)
				}
				field.SetInt(val)
			}

		case reflect.Float32, reflect.Float64:
			if field.Float() == 0 && defaultVal != "" {
				val, err := strconv.ParseFloat(defaultVal, 64)
				if err != nil {
					return fmt.Errorf("field %s: invalid float default value: %s (%w)",
						t.Field(i).Name, defaultVal, err)
				}
				field.SetFloat(val)
			}
		}
	}
	return nil
}

func extractDefaultFromTag(tag string) string {
	for _, part := range strings.Split(tag, ",") {
		if strings.HasPrefix(part, "default=") {
			return strings.TrimPrefix(part, "default=")
		}
	}
	return ""
}
