package config_test

import (
	"fmt"
	"os"
	"strings"

	"schneider.vip/config"
)

// DatabaseConfig is an example configuration struct.
type DatabaseConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

// GlobalConfig is a global config with some Subsections.
type GlobalConfig struct {
	DatabaseConfig DatabaseConfig `mapstructure:"databaseConfig"`
	HTTPListener   string
}

// ExampleNew demonstrates how to create a Config Loader from a file.
func ExampleNew() {
	loader := config.New[GlobalConfig](
		config.WithConfigFile[GlobalConfig]("internal/config.yml"),
	)

	config := loader.Load()
	fmt.Println("Database Host:", config.DatabaseConfig.Host)

	// Output: Database Host: localhost
}

// ExampleWithConfigReader demonstrates how to create a Config Loader from a reader.
func ExampleWithConfigReader() {
	configData := `{"host": "remote.example.com", "port": 5432}`
	reader := strings.NewReader(configData)

	loader := config.New[DatabaseConfig](
		config.WithConfigReader[DatabaseConfig](reader, "json"),
	)

	config := loader.Load()
	fmt.Println("Database Host:", config.Host)

	// Output: Database Host: remote.example.com
}

// ExampleDisableAutomaticEnv demonstrates how to disable automatic environment variables.
func ExampleDisableAutomaticEnv() {
	os.Setenv("DATABASECONFIG_HOST", "example.com")
	loader := config.New[GlobalConfig](
		config.WithConfigFile[GlobalConfig]("internal/config.yml"),
		config.DisableAutomaticEnv[GlobalConfig](),
	)

	config := loader.Load()
	fmt.Println("Database Host:", config.DatabaseConfig.Host)

	// Output: Database Host: localhost
}

// ExampleWithSubSection demonstrates how to load a specific subsection of the configuration.
func ExampleWithSubSection() {
	configData := `{"HTTPListener": "0.0.0.0:8888", "databaseConfig": {"host": "localhost", "port": 5432}}`

	loader := config.New[DatabaseConfig](
		config.WithConfigReader[DatabaseConfig](strings.NewReader(configData), "yaml"),
		config.WithSubSection[DatabaseConfig]("databaseConfig"),
	)

	config := loader.Load()
	fmt.Println("Database Host:", config.Host)

	// Output: Database Host: localhost
}

// ExampleStartDynamicReload demonstrates how to enable dynamic reloading of the configuration.
func ExampleStartDynamicReload() {
	loader := config.New[GlobalConfig](
		config.WithConfigFile[GlobalConfig]("internal/config.yml"),
	)

	loader.StartDynamicReload()

	config := loader.Load()
	fmt.Println("HTTP Listener:", config.HTTPListener)

	// Output: HTTP Listener: 0.0.0.0:8888
}
