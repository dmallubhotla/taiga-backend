package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config represents the application configuration
type Config struct {
	Port        string    `mapstructure:"port"`
	Environment string    `mapstructure:"environment"`
	Database    DB        `mapstructure:"database"`
	Migration   Migration `mapstructure:"migration"`
}

// DB holds database configuration
type DB struct {
	Driver   string `mapstructure:"driver"` // "postgres" or "sqlite"
	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Name     string `mapstructure:"name"`
	SSLMode  string `mapstructure:"sslmode"`
	FilePath string `mapstructure:"filepath"` // for sqlite
}

// Migration holds migration configuration
type Migration struct {
	Path     string `mapstructure:"path"`      // path to migration files
	AutoUp   bool   `mapstructure:"auto_up"`   // automatically run up migrations on startup
	AutoDown bool   `mapstructure:"auto_down"` // automatically run down migrations on shutdown (dev only)
}

// Load reads configuration from environment variables and config files
func Load() (*Config, error) {
	v := viper.New()

	// Set defaults
	v.SetDefault("port", "8080")
	v.SetDefault("environment", "development")
	v.SetDefault("database.driver", "sqlite")
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", "5432")
	v.SetDefault("database.user", "")
	v.SetDefault("database.password", "")
	v.SetDefault("database.name", "trygo")
	v.SetDefault("database.sslmode", "disable")
	v.SetDefault("database.filepath", "./data.db")
	v.SetDefault("migration.path", "./migrations")
	v.SetDefault("migration.auto_up", true)
	v.SetDefault("migration.auto_down", false)

	// Environment variable support
	v.SetEnvPrefix("TRYGO")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Allow environment variables to override config file
	// With AutomaticEnv and SetEnvPrefix, Viper automatically maps:
	// TRYGO_PORT -> port
	// TRYGO_ENVIRONMENT -> environment
	// TRYGO_DATABASE_DRIVER -> database.driver
	// TRYGO_DATABASE_HOST -> database.host
	// etc.

	// Try to read config file if it exists
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("./config")

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
		// Config file not found is OK, we'll use defaults and env vars
	}

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	return &config, nil
}

// DSN returns the database connection string
func (d *DB) DSN() string {
	switch d.Driver {
	case "postgres":
		return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
			d.Host, d.Port, d.User, d.Password, d.Name, d.SSLMode)
	case "sqlite":
		return d.FilePath
	default:
		return ""
	}
}

// IsDevelopment returns true if running in development mode
func (c *Config) IsDevelopment() bool {
	return c.Environment == "development" || c.Environment == "dev"
}

// IsProduction returns true if running in production mode
func (c *Config) IsProduction() bool {
	return c.Environment == "production" || c.Environment == "prod"
}
