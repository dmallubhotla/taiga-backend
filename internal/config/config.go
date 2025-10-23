package config

import (
	"fmt"
	"github.com/spf13/viper"
	"strings"
)

// Config represents the application configuration
type AppConfig struct {
	Port        string `mapstructure:"port"`
	Environment string `mapstructure:"environment"`
}

type TokensConfig struct {
	PrivateKeyPath string `mapstructure:"private_key_path"`
	PublicKeyPath  string `mapstructure:"public_key_path"`
}

// DB holds database configuration
type DBConfig struct {
	Driver        string `mapstructure:"driver"` // "postgres" or "sqlite"
	Host          string `mapstructure:"host"`
	Port          string `mapstructure:"port"`
	User          string `mapstructure:"user"`
	Password      string `mapstructure:"password"`
	Name          string `mapstructure:"name"`
	SSLMode       string `mapstructure:"ssl_mode"`
	FilePath      string `mapstructure:"filepath"`      // for sqlite
	DropOnStart   bool   `mapstructure:"drop_on_start"` // dev only
	AutoMigrateUp bool   `mapstructure:"auto_migrate_up"`
	MigrationPath string `mapstructure:"migration_path"`
}

type FileRepoConfig struct {
	AssetPath    string `mapstructure:"asset_path"`
	PrefixLength int    `mapstructure:"prefix_length"`
}

type Config struct {
	App      AppConfig      `mapstructure:"app"`
	Db       DBConfig       `mapstructure:"db"`
	Tokens   TokensConfig   `mapstructure:"tokens"`
	FileRepo FileRepoConfig `mapstructure:"file_repo"`
}

// Load reads configuration from environment variables and config files
// send in filename so we can support some args for it
func Load(filename string) (*Config, error) {
	v := viper.New()

	// Set defaults
	v.SetDefault("app.port", "8080")
	v.SetDefault("app.environment", "development")

	v.SetDefault("db.driver", "sqlite")
	v.SetDefault("db.host", "localhost")
	v.SetDefault("db.port", "5432")
	v.SetDefault("db.user", "")
	v.SetDefault("db.password", "")
	v.SetDefault("db.name", "trygo")
	v.SetDefault("db.ssl_mode", "disable")
	v.SetDefault("db.filepath", "./data.db")
	v.SetDefault("db.drop_on_start", false)
	v.SetDefault("db.auto_migrate_up", true)

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
