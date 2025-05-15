// Package config provides a generic configuration loader using viper.
// It supports loading configuration from files, readers, and environment variables.
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
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

// Loader is an interface for loading and parsing configuration.
type Loader[T any] interface {
	Parse() error
	Load() T
	StartWatcher() Dynamic[T]
}

// loader is a generic structure that loads and parses configuration.
type loader[T any] struct {
	config              atomic.Pointer[T] // Stores the current configuration
	viper               *viper.Viper      // Viper instance for configuration management
	disableAutomaticEnv bool
	withOnlyEnv         bool
	subSection          string
	onChangeCallback    func(error) // Callback function for change events
	disableAutoParse    bool        // Disables automatic parsing in New()
	logger              Logger      // Logger for logging messages
	useDefaultFilename  bool
	once                sync.Once // Ensure StartWatcher is called only once
	exampleConfig       string    // shown if Parse fails, to give user a sample copy&paste example config
	defaultConfig       T         // default config
	defaultConfigSet    bool
}

// Ensure loader implements Loader
var _ Loader[any] = (*loader[any])(nil)

// Option is a type for functional options.
type Option[T any] func(*loader[T])

// New creates a new Loader with functional options.
func New[T any](opts ...Option[T]) Loader[T] {
	// Create a new Viper instance with "_" as the key delimiter
	viperInstance := viper.NewWithOptions(viper.KeyDelimiter("_"))

	l := &loader[T]{
		config:              atomic.Pointer[T]{},
		viper:               viperInstance,
		disableAutomaticEnv: false,
		subSection:          "",
		onChangeCallback:    nil,
		disableAutoParse:    false,
		logger:              slogLogger{}, // Default to slog
		useDefaultFilename:  true,
		once:                sync.Once{},
	}

	// Apply functional options
	for _, opt := range opts {
		opt(l)
	}

	if l.useDefaultFilename {
		WithConfigFile[T]("config.yml")(l)
	}

	// Enable automatic environment variables
	if !l.disableAutomaticEnv {
		l.viper.AutomaticEnv()
	}

	// Parse the configuration initially unless disabled
	if !l.disableAutoParse {
		if err := l.Parse(); err != nil {
			if !l.defaultConfigSet {
				panic("Failed to load config: " + err.Error())
			}

			l.config.Store(&l.defaultConfig)
		}
	}

	return l
}

// WithOnlyEnv is an option to load configuration just a env.
func WithOnlyEnv[T any]() Option[T] {
	return func(cl *loader[T]) {
		cl.withOnlyEnv = true
		cl.viper.SetConfigFile("")
		cl.useDefaultFilename = false

		var cfg T

		cfgBytes, _ := json.Marshal(cfg)
		cl.viper.SetConfigType("json")

		if err := cl.viper.ReadConfig(bytes.NewReader(cfgBytes)); err != nil {
			cl.logger.Error("Failed to read config from reader", "error", err)
		}
	}
}

// WithConfigFile is an option to load configuration from a file.
// If WithConfigFile() and WithConfigReader() is not used, it will
// default to "config.yml".
func WithConfigFile[T any](configName string) Option[T] {
	return func(cl *loader[T]) {
		cl.useDefaultFilename = false
		cl.viper.SetConfigFile(configName)

		if err := cl.viper.ReadInConfig(); err != nil {
			cl.logger.Error("Failed to read config from file", "error", err)
		}
	}
}

// WithConfigReader is an option to load configuration from an io.Reader.
func WithConfigReader[T any](reader io.Reader, configType string) Option[T] {
	return func(cl *loader[T]) {
		cl.useDefaultFilename = false
		cl.viper.SetConfigType(configType)

		if err := cl.viper.ReadConfig(reader); err != nil {
			cl.logger.Error("Failed to read config from reader", "error", err)
		}
	}
}

// WithConfigPath adds config search Paths to viper before reading
func WithConfigPath[T any](configPaths []string) Option[T] {
	return func(cl *loader[T]) {
		for _, configPath := range configPaths {
			cl.viper.AddConfigPath(configPath)
		}

		if err := cl.viper.ReadInConfig(); err != nil {
			cl.logger.Error("Failed to read config from file", "error", err)
		}
	}
}

// WithViperInstance is an option to provide a custom Viper instance.
func WithViperInstance[T any](v *viper.Viper) Option[T] {
	return func(cl *loader[T]) {
		cl.viper = v
	}
}

// DisableAutomaticEnv is an option to disable automatic environment variable binding.
func DisableAutomaticEnv[T any]() Option[T] {
	return func(cl *loader[T]) {
		cl.disableAutomaticEnv = true
	}
}

// WithSubSection is an option to load only a SubSection.
func WithSubSection[T any](section string) Option[T] {
	return func(cl *loader[T]) {
		cl.subSection = section
	}
}

// WithOnChangeCallback is an option to set a callback function that is called when a change event occurs.
func WithOnChangeCallback[T any](callback func(error)) Option[T] {
	return func(cl *loader[T]) {
		cl.onChangeCallback = callback
	}
}

// WithExampleText is an option set an example config text which is shown if
// section was not found or some parsing error.
func WithExampleText[T any](example string) Option[T] {
	return func(cl *loader[T]) {
		cl.exampleConfig = example
	}
}

// WithDefault is an option set an default config to prevent Parse to panic.
// The default is only loaded if no config was found.
func WithDefault[T any](config T) Option[T] {
	return func(cl *loader[T]) {
		cl.defaultConfig = config
		cl.defaultConfigSet = true
	}
}

// DisableAutoParse is an option to disable automatic parsing in New(), this prevents panic when no config was found.
// The Parse() function needs to be called after New() and before Load().
func DisableAutoParse[T any]() Option[T] {
	return func(cl *loader[T]) {
		cl.disableAutoParse = true
	}
}

// WithLogger is an option to set a custom logger.
func WithLogger[T any](logger Logger) Option[T] {
	return func(cl *loader[T]) {
		cl.logger = logger
	}
}

var errSectionNotFound = errors.New("section not found in config")

// Parse parses the configuration it into the generic struct.
// If subsection set, only the specified subsection is parsed.
func (c *loader[T]) Parse() error {
	var config T

	var exampleText string
	if len(c.exampleConfig) > 0 {
		exampleText = fmt.Sprintf("\nExample Config:\n%s\n", c.exampleConfig)
	}

	// Extract the subsection if specified
	if c.subSection != "" {
		sub := c.viper.Sub(c.subSection)
		if sub == nil {
			return fmt.Errorf("%w: \"%s\"%s", errSectionNotFound, c.subSection, exampleText)
		}

		if err := sub.Unmarshal(&config); err != nil {
			return fmt.Errorf("failed to unmarshal section %s: %w%s", c.subSection, err, exampleText)
		}
	} else {
		// Parse the entire configuration
		if err := c.viper.Unmarshal(&config); err != nil {
			return fmt.Errorf("failed to unmarshal config: %w%s", err, exampleText)
		}
	}

	// Store the configuration in the atomic.Pointer
	c.config.Store(&config)

	return nil
}

// Load returns the latest parsed configuration.
func (c *loader[T]) Load() T {
	return *c.config.Load()
}

// Sets a new SetOnChangeFunc
func (c *loader[T]) SetOnChangeFunc(fn func(error)) {
	c.onChangeCallback = fn
	return
}

// StartWatcher starts a file watcher and parses the config on a change.
// Optional returns an dynamic conf Loader, but the loader[T] instance also can be used.
func (c *loader[T]) StartWatcher() Dynamic[T] {
	c.once.Do(func() {
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
	})

	return c
}

type Dynamic[T any] interface {
	Load() T
	SetOnChangeFunc(func(error))
}

// NewDynamic creates a new DynamicConf loader with functional options.
// Its Starts an background go routing by calling StartWatcher().
func NewDynamic[T any](opts ...Option[T]) (Dynamic[T], T) {
	dyn := New(opts...).StartWatcher()

	return dyn, dyn.Load()
}
