# Config

Config is a Go package for loading and managing configuration files using Viper. It supports:

- Loading configuration from files or readers.
- Automatic environment variable binding (can be disabled).
- Loading specific subsections of the configuration.
- Dynamic reloading of configuration files.

## Installation

```bash
go get schneider.vip/config
```

# Usage
## Loading from a File
```go
loader := config.New[GlobalConfig](
    config.WithConfigFile[GlobalConfig]("internal/config.yml"),
)

config := loader.Load()
fmt.Println("Database Host:", config.DatabaseConfig.Host)
```

## Loading from a Reader
```go
configData := `{"database": {"host": "localhost", "port": 5432}}`
reader := strings.NewReader(configData)

loader := config.New[DatabaseConfig](
    config.WithConfigReader[DatabaseConfig](reader, "json"),
)

config := loader.Load()
fmt.Println("Database Host:", config.Host)
```

## Automatic Environment Variables
```go
os.Setenv("DATABASECONFIG_HOST", "example.com")
loader := config.New[GlobalConfig](
    config.WithConfigFile[GlobalConfig]("internal/config.yml"),
)

config := loader.Load()
fmt.Println("Database Host:", config.DatabaseConfig.Host)
// Database Host: example.com
```

## Disabling Automatic Environment Variables
```go
os.Setenv("DATABASECONFIG_HOST", "example.com")
loader := config.New[GlobalConfig](
    config.WithConfigFile[GlobalConfig]("internal/config.yml"),
    config.DisableAutomaticEnv[GlobalConfig](),
)

config := loader.Load()
fmt.Println("Database Host:", config.DatabaseConfig.Host)
// The Host is still localhost from config.yml 
```

## Loading a Subsection

```go
loader := config.New[DatabaseConfig](
    config.WithConfigFile[DatabaseConfig]("config.yaml"),
    config.WithSubSection[DatabaseConfig]("database"),
)

config := loader.Load()
fmt.Println("Database Host:", config.Host)
```

## Dynamic Reloading

```go
loader := config.New[DatabaseConfig](
    config.WithConfigFile[DatabaseConfig]("config.yaml"),
)

loader.StartDynamicReload()

config := loader.Load()
fmt.Println("Database Host:", config.Host)
```

# Examples
See the examples for more usage patterns.

