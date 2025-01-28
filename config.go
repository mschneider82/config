// Package config provides a generic configuration loader using viper.
// It supports loading configuration from files, readers, and environment variables.
package config

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync/atomic"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

// Loader is a generic structure that loads and parses configuration.
type Loader[T any] struct {
	config              atomic.Pointer[T] // Stores the current configuration
	viper               *viper.Viper      // Viper instance for configuration management
	disableAutomaticEnv bool
	subSection          string
}

// Option is a type for functional options.
type Option[T any] func(*Loader[T])

// New creates a new ConfigLoader with functional options.
func New[T any](opts ...Option[T]) *Loader[T] {
	// Create a new Viper instance with "_" as the key delimiter
	viperInstance := viper.NewWithOptions(viper.KeyDelimiter("_"))

	loader := &Loader[T]{
		config:              atomic.Pointer[T]{},
		viper:               viperInstance,
		disableAutomaticEnv: false,
		subSection:          "",
	}

	// Apply functional options
	for _, opt := range opts {
		opt(loader)
	}

	// Enable automatic environment variables
	if !loader.disableAutomaticEnv {
		loader.viper.AutomaticEnv()
	}

	// Parses the configuration initially
	if err := loader.Parse(); err != nil {
		panic("Failed to load config: " + err.Error())
	}

	return loader
}

// WithConfigFile is an option to load configuration from a file.
func WithConfigFile[T any](configPath string) Option[T] {
	return func(cl *Loader[T]) {
		cl.viper.SetConfigFile(configPath)

		if err := cl.viper.ReadInConfig(); err != nil {
			slog.Error("Failed to read config from file", "error", err)
		}
	}
}

// WithConfigReader is an option to load configuration from an io.Reader.
func WithConfigReader[T any](reader io.Reader, configType string) Option[T] {
	return func(cl *Loader[T]) {
		cl.viper.SetConfigType(configType)

		if err := cl.viper.ReadConfig(reader); err != nil {
			slog.Error("Failed to read config from reader", "error", err)
		}
	}
}

// WithViperInstance is an option to provide a custom Viper instance.
func WithViperInstance[T any](v *viper.Viper) Option[T] {
	return func(cl *Loader[T]) {
		cl.viper = v
	}
}

// DisableAutomaticEnv is an option to disable automatic environment variable binding.
func DisableAutomaticEnv[T any]() Option[T] {
	return func(cl *Loader[T]) {
		cl.disableAutomaticEnv = true
	}
}

// WithSubSection is an option to load only a SubSection.
func WithSubSection[T any](section string) Option[T] {
	return func(cl *Loader[T]) {
		cl.subSection = section
	}
}

var errSectionNotFound = errors.New("section not found in config")

// Parse parses the configuration it into the generic struct.
// If subsection set, only the specified subsection is parsed.
func (c *Loader[T]) Parse() error {
	var config T

	// Extract the subsection if specified
	if c.subSection != "" {
		sub := c.viper.Sub(c.subSection)
		if sub == nil {
			return fmt.Errorf("%w: %s", errSectionNotFound, c.subSection)
		}

		if err := sub.Unmarshal(&config); err != nil {
			return fmt.Errorf("failed to unmarshal section %s: %w", c.subSection, err)
		}
	} else {
		// Parse the entire configuration
		if err := c.viper.Unmarshal(&config); err != nil {
			return fmt.Errorf("failed to unmarshal config: %w", err)
		}
	}

	// Store the configuration in the atomic.Pointer
	c.config.Store(&config)

	return nil
}

// Load returns the current configuration.
func (c *Loader[T]) Load() T {
	return *c.config.Load()
}

// StartDynamicReload starts a file watcher and parses the config on a change.
func (c *Loader[T]) StartDynamicReload() {
	// Register a callback for configuration changes
	c.viper.OnConfigChange(func(event fsnotify.Event) {
		if err := c.Parse(); err != nil { // Section is passed here
			slog.Error("Failed to reload config", "error", err)
		} else {
			slog.Info("Config reloaded successfully")
		}
	})

	go func() {
		// Enable watching for file changes
		c.viper.WatchConfig()
	}()
}
