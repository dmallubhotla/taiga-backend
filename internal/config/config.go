package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config represents the application configuration
type AppConfig struct {
	Port        string
	Environment string
	// Database    DB
	// Migration   Migration `mapstructure:"migration"`
}

// DB holds database configuration
type DBConfig struct {
	Driver     string //`mapstructure:"driver"` // "postgres" or "sqlite"
	Host       string //`mapstructure:"host"`
	Port       string //`mapstructure:"port"`
	User       string //`mapstructure:"user"`
	Password   string //`mapstructure:"password"`
	Name       string //`mapstructure:"name"`
	SSLMode    string // `mapstructure`: "ssl_mode"
	FilePath   string //`mapstructure:"filepath"` // for sqlite
	DropOnStat bool   //`mapstructure:"drop_on_stat"`
}

type Config struct {
	App AppConfig //`mapstructure: "app"`
	Db  DBConfig  //`mapstructure: "db"`
}

// Migration holds migration configuration
// type Migration struct {
// 	Path     string //`mapstructure:"path"`      // path to migration files
// 	AutoUp   bool   //`mapstructure:"auto_up"`   // automatically run up migrations on startup
// 	AutoDown bool   //`mapstructure:"auto_down"` // automatically run down migrations on shutdown (dev only)
// }

// Load reads configuration from environment variables and config files
// send in filename so we can support some args for it
func Load(filename string) (*Config, error) {
	v := viper.New()

	// Set defaults
	v.SetDefault("app.port", "8080")
	v.SetDefault("app.environment", "development")

	v.SetDefault("database.driver", "sqlite")
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", "5432")
	v.SetDefault("database.user", "")
	v.SetDefault("database.password", "")
	v.SetDefault("database.name", "trygo")
	v.SetDefault("database.ssl_mode", "disable")
	v.SetDefault("database.filepath", "./data.db")
	v.SetDefault("database.drop_on_start", false)

	// Environment variable support
	v.SetEnvPrefix("TRYGO")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Try to read config file if it exists
	v.SetConfigName(filename)
	v.SetConfigType("yaml")
	v.AddConfigPath(".")

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
func (d *DBConfig) DSN() string {
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
func (a *AppConfig) IsDevelopment() bool {
	return a.Environment == "development" || a.Environment == "dev"
}

// IsProduction returns true if running in production mode
func (a *AppConfig) IsProduction() bool {
	return a.Environment == "production" || a.Environment == "prod"
}
