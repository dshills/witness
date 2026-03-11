package app

import "github.com/dshills/witness/internal/config"

// App holds shared application state.
type App struct {
	Config *config.Config
}

// New creates a new App with the given config.
func New(cfg *config.Config) *App {
	return &App{Config: cfg}
}
