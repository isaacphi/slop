package config

import (
	"fmt"
	"reflect"
	"strings"
)

// ConfigSchema defines the configuration structure
type ConfigSchema struct {
	Models      []string `mapstructure:"models" validate:"required"`
	MainModel   string   `mapstructure:"mainModel"`
	Prompts     []string `mapstructure:"prompts"`
	MaxTokens   int      `mapstructure:"maxTokens" validate:"required"`
	Temperature float64  `mapstructure:"temperature" validate:"required"`
	LLMKey      string   `mapstructure:"llm_key"`
	DBPath      string   `mapstructure:"DBPath"`

	// Internal fields for printing
	sources  map[string][]configSource
	defaults map[string]interface{}
}

// GetKnownKeys returns all valid configuration keys based on the schema
func GetKnownKeys() map[string]bool {
	known := make(map[string]bool)
	t := reflect.TypeOf(ConfigSchema{})

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if tag := field.Tag.Get("mapstructure"); tag != "" {
			known[strings.ToLower(tag)] = true
		}
	}
	return known
}

// PrintConfig prints the configuration with optional defaults and sources
func (s *ConfigSchema) PrintConfig(includeDefaults bool, includeSources bool) {
	fmt.Println("Configuration:")
	t := reflect.TypeOf(*s)
	v := reflect.ValueOf(*s)

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		value := v.Field(i)

		// Skip internal fields and zero values
		if !field.IsExported() || value.IsZero() {
			continue
		}

		tag := field.Tag.Get("mapstructure")
		if tag == "" {
			continue
		}

		s.printField(tag, value.Interface(), includeDefaults, includeSources)
	}
}

func (s *ConfigSchema) printField(key string, value interface{}, includeDefaults, includeSources bool) {
	// Handle slices
	if slice, ok := value.([]string); ok {
		fmt.Printf("%s:\n", key)
		for _, item := range slice {
			fmt.Printf("  - %v", item)
			s.printSourceInfo(key, item, includeDefaults, includeSources)
			fmt.Println()
		}
		return
	}

	// Handle sensitive values
	if isSecretKey(key) {
		fmt.Printf("%s: [REDACTED]", key)
	} else {
		fmt.Printf("%s: %v", key, value)
	}

	// Print source/default info for non-slice values
	s.printSourceInfo(key, value, includeDefaults, includeSources)
	fmt.Println()
}

func (s *ConfigSchema) printSourceInfo(key string, value interface{}, includeDefaults, includeSources bool) {
	if !includeSources {
		return
	}

	// Try both original case and lowercase key when looking up sources
	sources, ok := s.sources[key]
	if !ok {
		sources = s.sources[strings.ToLower(key)]
	}

	if len(sources) > 0 {
		// For slice items, look for matching source
		if strValue, ok := value.(string); ok {
			for _, src := range sources {
				if srcSlice, ok := src.value.([]interface{}); ok {
					for _, srcItem := range srcSlice {
						if fmt.Sprintf("%v", srcItem) == strValue {
							fmt.Printf(" (%s)", src.source)
							return
						}
					}
				}
			}
		} else {
			// For non-slice values, use the last source
			fmt.Printf(" (%s)", sources[len(sources)-1].source)
		}
	} else if includeDefaults && s.defaults != nil {
		// Try both original case and lowercase for defaults too
		defaultVal, ok := s.defaults[key]
		if !ok {
			defaultVal = s.defaults[strings.ToLower(key)]
		}
		if ok && reflect.DeepEqual(value, defaultVal) {
			fmt.Printf(" (default)")
		}
	}
}

func isSecretKey(key string) bool {
	return strings.Contains(strings.ToLower(key), "key") ||
		strings.Contains(strings.ToLower(key), "secret") ||
		strings.Contains(strings.ToLower(key), "password")
}
