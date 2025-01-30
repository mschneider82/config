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

// Logger is a simple interface for logging.
type Logger interface {
	Info(msg string, args ...any)
	Error(msg string, args ...any)
}

// slogLogger is a wrapper around slog to implement the Logger interface.
type slogLogger struct{}

func (s slogLogger) Info(msg string, args ...any) {
	slog.Info(msg, args...)
}

func (s slogLogger) Error(msg string, args ...any) {
	slog.Error(msg, args...)
}

// Loader is a generic structure that loads and parses configuration.
type Loader[T any] struct {
	config              atomic.Pointer[T] // Stores the current configuration
	viper               *viper.Viper      // Viper instance for configuration management
	disableAutomaticEnv bool
	subSection          string
	onChangeCallback    func(error) // Callback function for change events
	disableAutoParse    bool        // Disables automatic parsing in New()
	logger              Logger      // Logger for logging messages
	useDefaultFilename  bool
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
		onChangeCallback:    nil,
		disableAutoParse:    false,
		logger:              slogLogger{}, // Default to slog
		useDefaultFilename:  true,
	}

	// Apply functional options
	for _, opt := range opts {
		opt(loader)
	}

	if loader.useDefaultFilename {
		WithConfigFile[T]("config.yml")(loader)
	}

	// Enable automatic environment variables
	if !loader.disableAutomaticEnv {
		loader.viper.AutomaticEnv()
	}

	// Parse the configuration initially unless disabled
	if !loader.disableAutoParse {
		if err := loader.Parse(); err != nil {
			panic("Failed to load config: " + err.Error())
		}
	}

	return loader
}

// WithConfigFile is an option to load configuration from a file.
// If WithConfigFile() and WithConfigReader() is not used, it will
// default to "config.yml".
func WithConfigFile[T any](configPath string) Option[T] {
	return func(cl *Loader[T]) {
		cl.useDefaultFilename = false
		cl.viper.SetConfigFile(configPath)

		if err := cl.viper.ReadInConfig(); err != nil {
			cl.logger.Error("Failed to read config from file", "error", err)
		}
	}
}

// WithConfigReader is an option to load configuration from an io.Reader.
func WithConfigReader[T any](reader io.Reader, configType string) Option[T] {
	return func(cl *Loader[T]) {
		cl.useDefaultFilename = false
		cl.viper.SetConfigType(configType)

		if err := cl.viper.ReadConfig(reader); err != nil {
			cl.logger.Error("Failed to read config from reader", "error", err)
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

// WithOnChangeCallback is an option to set a callback function that is called when a change event occurs.
func WithOnChangeCallback[T any](callback func(error)) Option[T] {
	return func(cl *Loader[T]) {
		cl.onChangeCallback = callback
	}
}

// DisableAutoParse is an option to disable automatic parsing in New(), this prevents panic when no config was found.
// The Parse() function needs to be called after New() and before Load().
func DisableAutoParse[T any]() Option[T] {
	return func(cl *Loader[T]) {
		cl.disableAutoParse = true
	}
}

// WithLogger is an option to set a custom logger.
func WithLogger[T any](logger Logger) Option[T] {
	return func(cl *Loader[T]) {
		cl.logger = logger
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

// Load returns the latest parsed configuration.
func (c *Loader[T]) Load() T {
	return *c.config.Load()
}

// StartWatcher starts a file watcher and parses the config on a change.
// Optional returns an dynamic conf Loader, but the Loader[T] instance also can be used.
func (c *Loader[T]) StartWatcher() Dynamic[T] {
	// Register a callback for configuration changes
	c.viper.OnConfigChange(func(event fsnotify.Event) {
		err := c.Parse() // Section is passed here
		if err != nil {
			c.logger.Error("Failed to reload config", "error", err)
		} else {
			c.logger.Info("Config reloaded successfully")
		}

		if c.onChangeCallback != nil {
			c.onChangeCallback(err) // Call the callback function with the error (if any)
		}
	})

	go func() {
		// Enable watching for file changes
		c.viper.WatchConfig()
	}()

	return c
}

type Dynamic[T any] interface {
	Load() T
}

// NewDynamic creates a new DynamicConf loader with functional options.
// Its Starts an background go routing by calling StartWatcher().
func NewDynamic[T any](opts ...Option[T]) (Dynamic[T], T) {
	dynloader := New(opts...)
	dynloader.StartWatcher()

	return dynloader, dynloader.Load()
}
