package config

import (
	"fmt"
	"reflect"
	"strings"
)

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

// PrintConfig prints the configuration with optional sources
func (s *ConfigSchema) PrintConfig(includeSources bool) {
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

		s.printField(tag, value.Interface(), includeSources)
	}
}

func (s *ConfigSchema) printField(key string, value interface{}, includeSources bool) {
	// Handle slices
	if slice, ok := value.([]string); ok {
		fmt.Printf("%s:\n", key)
		for _, item := range slice {
			fmt.Printf("  - %v", item)
			s.printSourceInfo(key, item, includeSources)
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
	s.printSourceInfo(key, value, includeSources)
	fmt.Println()
}

func (s *ConfigSchema) printSourceInfo(key string, value interface{}, includeSources bool) {
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
	}
}

func isSecretKey(key string) bool {
	return strings.Contains(strings.ToLower(key), "key") ||
		strings.Contains(strings.ToLower(key), "secret") ||
		strings.Contains(strings.ToLower(key), "password")
}
