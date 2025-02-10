package config

import (
	"fmt"
	"reflect"
	"strings"
)

// PrintConfig prints the configuration with optional sources in YAML format
func (s *ConfigSchema) PrintConfig(includeSources bool, prefix string) {
	s.printValue(reflect.ValueOf(*s), "", "", includeSources, 0, prefix)
	for _, w := range s.warnings {
		fmt.Println(w)
	}
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
