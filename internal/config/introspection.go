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

		// Recursively handle struct fields
		if field.Type.Kind() == reflect.Struct {
			// Create a zero value of the field type to recurse into
			addKnowKeysByValue(key, reflect.New(field.Type).Elem().Interface(), known)
		}

		// Handle different types
		switch field.Type.Kind() {
		case reflect.Map:
			// Add base map key
			known[key] = true

			// For maps of any type, we need to handle possible nesting
			elemType := field.Type.Elem()
			wildcardPrefix := fmt.Sprintf("%s.*", key)
			known[wildcardPrefix] = true

			// If it's a map to a struct, recursively add its fields
			if elemType.Kind() == reflect.Struct {
				// Create an instance of the map value type to recurse into
				elemValue := reflect.New(elemType).Elem().Interface()
				addKnowKeysByValue(wildcardPrefix, elemValue, known)
			} else if elemType.Kind() == reflect.Map {
				// For nested maps, recurse with the wildcard prefix
				// This handles cases like map[string]map[string]Property
				subElemType := elemType.Elem()
				if subElemType.Kind() == reflect.Struct {
					subElemValue := reflect.New(subElemType).Elem().Interface()
					addKnowKeysByValue(wildcardPrefix, subElemValue, known)
				}
			}

		case reflect.Slice, reflect.Array:
			// Add the base slice/array key
			known[key] = true
			// For slices/arrays of structs, add their fields
			if field.Type.Elem().Kind() == reflect.Struct {
				elemType := field.Type.Elem()
				wildcardPrefix := fmt.Sprintf("%s.[*]", key)
				elemValue := reflect.New(elemType).Elem().Interface()
				addKnowKeysByValue(wildcardPrefix, elemValue, known)
			} else {
				// For simple slices, allow any index
				wildcardKey := fmt.Sprintf("%s.[*]", key)
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
func (s *ConfigSchema) PrintConfig(includeSources bool, prefix string) {
	s.printValue(reflect.ValueOf(*s), "", "", includeSources, 0, prefix)
}

func (s *ConfigSchema) printValue(v reflect.Value, key, fullKey string, includeSources bool, indent int, prefix string) {
	t := v.Type()

	prefixParts := strings.Split(prefix, ".")
	prefixNext := ""
	prefixPart := prefixParts[0]
	if len(prefixParts) > 0 {
		prefixNext = strings.Join(prefixParts[1:], ".")
	}

	switch v.Kind() {
	case reflect.Struct:
		if key != "" {
			fmt.Printf("%s%s:\n", strings.Repeat("  ", indent), key)
			indent++
		}
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			if !strings.HasPrefix(strings.ToLower(field.Name), prefixPart) {
				continue
			}
			if !field.IsExported() || field.Tag.Get("mapstructure") == "" {
				continue
			}
			fieldValue := v.Field(i)
			if !fieldValue.IsZero() {
				tag := field.Tag.Get("mapstructure")
				var nextFullKey string
				if fullKey == "" {
					nextFullKey = tag
				} else {
					nextFullKey = fullKey + "." + tag
				}
				s.printValue(fieldValue, tag, nextFullKey, includeSources, indent, prefixNext)
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
			if !strings.HasPrefix(strings.ToLower(k), prefixPart) {
				continue
			}
			var nextFullKey string
			if fullKey == "" {
				nextFullKey = k
			} else {
				nextFullKey = fullKey + "." + k
			}
			s.printValue(iter.Value(), k, nextFullKey, includeSources, indent, prefixNext)
		}

	default:
		if isSecretKey(key) {
			fmt.Printf("%s%s: [REDACTED]", strings.Repeat("  ", indent), key)
		} else {
			fmt.Printf("%s%s: %v", strings.Repeat("  ", indent), key, v.Interface())
		}
		s.printSourceInfo(fullKey, includeSources)
		fmt.Println()
	}
}

func (s *ConfigSchema) printSourceInfo(key string, includeSources bool) {
	if !includeSources {
		return
	}

	if source, ok := s.sources[strings.ToLower(key)]; ok {
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
