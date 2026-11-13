package config

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"
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
	Type         string `mapstructure:"type"`          // "local" or "s3"
	AssetPath    string `mapstructure:"asset_path"`    // for local storage
	PrefixLength int    `mapstructure:"prefix_length"` // directory structure depth
	S3Bucket     string `mapstructure:"s3_bucket"`     // S3 bucket name
	S3Region     string `mapstructure:"s3_region"`     // S3 region
	S3Prefix     string `mapstructure:"s3_prefix"`     // S3 key prefix
}

type AuthConfig struct {
	// "open" or "closed"; empty means open in development and closed everywhere else.
	Registration string `mapstructure:"registration"`
	// False turns off /auth/register and /auth/tokens, for a deployment that signs in only through OIDC.
	PasswordLogin bool `mapstructure:"password_login"`
	// The frontend's origin as the browser sees it; a provider sign-in ends with a redirect there.
	PublicBaseURL string `mapstructure:"public_base_url"`
	// Where the browser reaches this API, for the callback URL registered with each provider; defaults to public_base_url + "/api", the frontend's proxy.
	APIBaseURL string         `mapstructure:"api_base_url"`
	OIDC       []OIDCProvider `mapstructure:"oidc"`
}

// OIDCProvider is one OpenID Connect provider to offer at sign-in; its endpoints come from the issuer's discovery document.
// The json tags let TAIGA_AUTH_OIDC hold the list as JSON.
type OIDCProvider struct {
	// Short and URL-safe: it's in the callback path.
	Name             string   `mapstructure:"name" json:"name"`
	DisplayName      string   `mapstructure:"display_name" json:"display_name"`
	Issuer           string   `mapstructure:"issuer" json:"issuer"`
	ClientID         string   `mapstructure:"client_id" json:"client_id"`
	ClientSecretFile string   `mapstructure:"client_secret_file" json:"client_secret_file"`
	Scopes           []string `mapstructure:"scopes" json:"scopes"`
	// Nil means true: an unknown identity gets a new account.
	AllowSignup *bool `mapstructure:"allow_signup" json:"allow_signup"`
	// Sign an unknown identity into the account with the same email, only when the provider says the email is verified.
	LinkByVerifiedEmail bool `mapstructure:"link_by_verified_email" json:"link_by_verified_email"`
}

type Config struct {
	App      AppConfig      `mapstructure:"app"`
	Auth     AuthConfig     `mapstructure:"auth"`
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
	v.SetDefault("db.name", "taiga")
	v.SetDefault("db.ssl_mode", "disable")
	v.SetDefault("db.filepath", "./data.db")
	v.SetDefault("db.drop_on_start", false)
	v.SetDefault("db.auto_migrate_up", true)

	v.SetDefault("file_repo.type", "local")
	v.SetDefault("file_repo.asset_path", "local/filerepo/")
	v.SetDefault("file_repo.prefix_length", 2)

	v.SetDefault("auth.password_login", true)

	// Environment variable support
	v.SetEnvPrefix("TAIGA")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	// AutomaticEnv only reaches keys viper already knows, so a key with no default (e.g. db.migration_path) would ignore its env var unless bound.
	bindEnvs(v, reflect.TypeFor[Config](), "")

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
	if err := v.Unmarshal(&config, viper.DecodeHook(mapstructure.ComposeDecodeHookFunc(
		mapstructure.StringToTimeDurationHookFunc(),
		mapstructure.StringToSliceHookFunc(","),
		oidcFromJSON,
	))); err != nil {
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

// RegistrationOpen reports whether POST /auth/register accepts new users.
// An unrecognised value is an error rather than a guess, so a typo can't leave registration open.
func (c *Config) RegistrationOpen() (bool, error) {
	switch c.Auth.Registration {
	case "":
		return c.App.IsDevelopment(), nil
	case "open":
		return true, nil
	case "closed":
		return false, nil
	default:
		return false, fmt.Errorf("auth.registration must be \"open\" or \"closed\", got %q", c.Auth.Registration)
	}
}

// APIBase is where the browser reaches this API, without a trailing slash.
func (a *AuthConfig) APIBase() string {
	if a.APIBaseURL != "" {
		return strings.TrimSuffix(a.APIBaseURL, "/")
	}
	return strings.TrimSuffix(a.PublicBaseURL, "/") + "/api"
}

// oidcFromJSON reads the provider list from an env var, which can only hold a string.
func oidcFromJSON(from reflect.Type, to reflect.Type, data any) (any, error) {
	if from.Kind() != reflect.String || to != reflect.TypeFor[[]OIDCProvider]() {
		return data, nil
	}
	var providers []OIDCProvider
	if err := json.Unmarshal([]byte(data.(string)), &providers); err != nil {
		return nil, fmt.Errorf("auth.oidc from the environment must be a JSON list of providers: %w", err)
	}
	return providers, nil
}

// IsProduction returns true if running in production mode
func (a *AppConfig) IsProduction() bool {
	return a.Environment == "production" || a.Environment == "prod"
}

// bindEnvs binds every mapstructure key in t, so each config field can be set from TAIGA_<SECTION>_<KEY>.
func bindEnvs(v *viper.Viper, t reflect.Type, prefix string) {
	for field := range t.Fields() {
		key := field.Tag.Get("mapstructure")
		if key == "" {
			continue
		}
		if prefix != "" {
			key = prefix + "." + key
		}
		if field.Type.Kind() == reflect.Struct {
			bindEnvs(v, field.Type, key)
			continue
		}
		_ = v.BindEnv(key)
	}
}
