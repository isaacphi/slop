package config

import (
	"fmt"
	"reflect"
	"strings"
)

// GetKnownKeys returns all valid configuration keys based on the schema
func GetKnownKeys() map[string]bool {
	known := make(map[string]bool)
	addKnowKeysByValue("", ConfigSchema{}, known)
	return known
}

// addKnowKeysByValue recursively adds keys by examining struct value
func addKnowKeysByValue(prefix string, val interface{}, known map[string]bool) {
	v := reflect.ValueOf(val)
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}

		tag := field.Tag.Get("mapstructure")
		if tag == "" {
			continue
		}

		key := tag
		if prefix != "" {
			key = prefix + "." + tag
		}

		// Convert the key to lowercase since viper lowercases all keys
		key = strings.ToLower(key)
		known[key] = true

		// Add wildcard entries for maps
		switch field.Type.Kind() {
		case reflect.Map:
			// Add base map key
			known[key] = true

			// For maps of structs, add their fields
			if field.Type.Elem().Kind() == reflect.Struct {
				elemType := field.Type.Elem()
				for j := 0; j < elemType.NumField(); j++ {
					subField := elemType.Field(j)
					if subTag := subField.Tag.Get("mapstructure"); subTag != "" {
						wildcardKey := fmt.Sprintf("%s.*.%s", key, strings.ToLower(subTag))
						known[wildcardKey] = true
					}
				}
			} else {
				// For simple maps (like theme), allow any nested fields
				wildcardKey := fmt.Sprintf("%s.*", key)
				known[wildcardKey] = true
			}
		}
	}
}

// matchesWildcard checks if a key matches a wildcard pattern
func matchesWildcard(pattern, key string) bool {
	// Convert both to lowercase for case-insensitive matching
	pattern = strings.ToLower(pattern)
	key = strings.ToLower(key)

	// Split into parts
	patternParts := strings.Split(pattern, ".")
	keyParts := strings.Split(key, ".")

	// Must have same number of parts
	if len(patternParts) != len(keyParts) {
		return false
	}

	// Check each part
	for i := range patternParts {
		if patternParts[i] != "*" && patternParts[i] != keyParts[i] {
			return false
		}
	}
	return true
}

// IsKnownKey checks if a key is known, including wildcard matches
func IsKnownKey(known map[string]bool, key string) bool {
	// Check direct match first
	if known[strings.ToLower(key)] {
		return true
	}

	// Check wildcard patterns
	for pattern := range known {
		if strings.Contains(pattern, "*") && matchesWildcard(pattern, key) {
			return true
		}
	}
	return false
}

// PrintConfig prints the configuration with optional sources in YAML format
func (s *ConfigSchema) PrintConfig(includeSources bool) {
	s.printValue(reflect.ValueOf(*s), "", includeSources, 0)
}

func (s *ConfigSchema) printValue(v reflect.Value, key string, includeSources bool, indent int) {
	t := v.Type()

	switch v.Kind() {
	case reflect.Struct:
		if key != "" {
			fmt.Printf("%s%s:\n", strings.Repeat("  ", indent), key)
			indent++
		}
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			if !field.IsExported() || field.Tag.Get("mapstructure") == "" {
				continue
			}
			fieldValue := v.Field(i)
			if !fieldValue.IsZero() {
				tag := field.Tag.Get("mapstructure")
				s.printValue(fieldValue, tag, includeSources, indent)
			}
		}

	case reflect.Map:
		if key != "" {
			fmt.Printf("%s%s:\n", strings.Repeat("  ", indent), key)
			indent++
		}
		iter := v.MapRange()
		for iter.Next() {
			k := iter.Key().String()
			s.printValue(iter.Value(), k, includeSources, indent)
		}

	default:
		if isSecretKey(key) {
			fmt.Printf("%s%s: [REDACTED]", strings.Repeat("  ", indent), key)
		} else {
			fmt.Printf("%s%s: %v", strings.Repeat("  ", indent), key, v.Interface())
		}
		s.printSourceInfo(key, includeSources)
		fmt.Println()
	}
}

func (s *ConfigSchema) printSourceInfo(key string, includeSources bool) {
	if !includeSources {
		return
	}

	// Build full key path for nested fields
	fullKey := key
	found := false
	var source string

	// Check direct key first
	if sources, ok := s.sources[fullKey]; ok && len(sources) > 0 {
		source = sources[len(sources)-1].source
		found = true
	}

	// If not found, try lowercase key
	if !found {
		if sources, ok := s.sources[strings.ToLower(fullKey)]; ok && len(sources) > 0 {
			source = sources[len(sources)-1].source
			found = true
		}
	}

	// Print source information
	if found {
		fmt.Printf(" # (%s)", source)
	} else {
		fmt.Printf(" # (default)")
	}
}

func isSecretKey(key string) bool {
	return strings.Contains(strings.ToLower(key), "key") ||
		strings.Contains(strings.ToLower(key), "secret") ||
		strings.Contains(strings.ToLower(key), "password")
}
