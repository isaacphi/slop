package config

import "fmt"

// RuntimeOverrides holds configuration values that can be overridden at runtime
// via CLI flags or other means
type RuntimeOverrides struct {
	ActiveModel *string
	MaxTokens   *int
	Temperature *float64
}

// NewConfigWithOverrides creates a new config and applies any runtime overrides
func NewConfigWithOverrides(overrides *RuntimeOverrides) (*ConfigSchema, error) {
	cfg, err := New()
	if err != nil {
		return nil, err
	}

	if overrides != nil {
		if overrides.ActiveModel != nil {
			if _, exists := cfg.Models[*overrides.ActiveModel]; !exists {
				return nil, fmt.Errorf("model %q not found in configuration", *overrides.ActiveModel)
			}
			cfg.ActiveModel = *overrides.ActiveModel
		}

		activeModel := cfg.Models[cfg.ActiveModel]
		if overrides.MaxTokens != nil {
			activeModel.MaxTokens = *overrides.MaxTokens
		}
		if overrides.Temperature != nil {
			activeModel.Temperature = *overrides.Temperature
		}
		cfg.Models[cfg.ActiveModel] = activeModel
	}

	return cfg, nil
}
