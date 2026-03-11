package config

import "context"

type configKey struct{}

// WithConfig returns a new context with the given config.
func WithConfig(ctx context.Context, cfg *Config) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, configKey{}, cfg)
}

// FromContext retrieves the config from the context, or nil.
func FromContext(ctx context.Context) *Config {
	if ctx == nil {
		return nil
	}
	cfg, _ := ctx.Value(configKey{}).(*Config)
	return cfg
}
