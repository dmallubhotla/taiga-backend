package config_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitea.deepak.science/deepak/taiga/internal/config"
)

func TestLoadDefaults(t *testing.T) {
	// Clear any existing environment variables
	clearEnvVars(t)

	// Change to testdata directory temporarily
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()

	// Change to a directory without config files to test defaults
	tempDir := t.TempDir()
	err = os.Chdir(tempDir)
	require.NoError(t, err)

	cfg, err := config.Load("config")
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Test default values
	assert.Equal(t, "8080", cfg.App.Port)
	assert.Equal(t, "development", cfg.App.Environment)
	assert.Equal(t, "sqlite", cfg.Db.Driver)
	assert.Equal(t, "localhost", cfg.Db.Host)
	assert.Equal(t, "5432", cfg.Db.Port)
	assert.Equal(t, "taiga", cfg.Db.Name)
	assert.Equal(t, "disable", cfg.Db.SSLMode)
	assert.Equal(t, "./data.db", cfg.Db.FilePath)
}

func TestLoadFullConfig(t *testing.T) {
	clearEnvVars(t)

	// Change to testdata directory
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()

	err = os.Chdir("testdata")
	require.NoError(t, err)

	// Copy config-full.yaml to config.yaml
	copyFile(t, "config-full.yaml", "config.yaml")
	defer func() { _ = os.Remove("config.yaml") }()

	cfg, err := config.Load("config")
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Test loaded values
	assert.Equal(t, "9090", cfg.App.Port)
	assert.Equal(t, "production", cfg.App.Environment)
	assert.Equal(t, "postgres", cfg.Db.Driver)
	assert.Equal(t, "db.example.com", cfg.Db.Host)
	assert.Equal(t, "5433", cfg.Db.Port)
	assert.Equal(t, "testuser", cfg.Db.User)
	assert.Equal(t, "testpass", cfg.Db.Password)
	assert.Equal(t, "testdb", cfg.Db.Name)
	assert.Equal(t, "require", cfg.Db.SSLMode)
	assert.Equal(t, "/custom/path.db", cfg.Db.FilePath)
}

func TestLoadPartialConfig(t *testing.T) {
	clearEnvVars(t)

	// Change to testdata directory
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()

	err = os.Chdir("testdata")
	require.NoError(t, err)

	// Copy config-partial.yaml to config.yaml
	copyFile(t, "config-partial.yaml", "config.yaml")
	defer func() { _ = os.Remove("config.yaml") }()

	cfg, err := config.Load("config")
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Test partial config with defaults
	assert.Equal(t, "3000", cfg.App.Port)
	assert.Equal(t, "development", cfg.App.Environment) // default
	assert.Equal(t, "sqlite", cfg.Db.Driver)
	assert.Equal(t, "localhost", cfg.Db.Host) // default
	assert.Equal(t, "5432", cfg.Db.Port)      // default
	assert.Equal(t, "./test.db", cfg.Db.FilePath)
}

func TestEnvironmentVariables(t *testing.T) {
	clearEnvVars(t)

	// Set environment variables
	t.Setenv("TAIGA_APP_PORT", "7777")
	t.Setenv("TAIGA_APP_ENVIRONMENT", "testing")
	t.Setenv("TAIGA_DB_DRIVER", "postgres")
	t.Setenv("TAIGA_DB_HOST", "env-db-host")
	t.Setenv("TAIGA_DB_USER", "env-user")

	// Change to a directory without config files
	tempDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()

	err = os.Chdir(tempDir)
	require.NoError(t, err)

	cfg, err := config.Load("config")
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Environment variables should override defaults
	assert.Equal(t, "7777", cfg.App.Port)
	assert.Equal(t, "testing", cfg.App.Environment)
	assert.Equal(t, "postgres", cfg.Db.Driver)
	assert.Equal(t, "env-db-host", cfg.Db.Host)
	assert.Equal(t, "env-user", cfg.Db.User)
	// Defaults should still apply for unset env vars
	assert.Equal(t, "5432", cfg.Db.Port)
	assert.Equal(t, "taiga", cfg.Db.Name)
}

func TestEnvironmentVariablesOverrideConfig(t *testing.T) {
	clearEnvVars(t)

	// Set environment variables that should override config file
	t.Setenv("TAIGA_APP_PORT", "8888")
	t.Setenv("TAIGA_DB_DRIVER", "sqlite")

	// Change to testdata directory
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()

	err = os.Chdir("testdata")
	require.NoError(t, err)

	// Copy config-full.yaml to config.yaml
	copyFile(t, "config-full.yaml", "config.yaml")
	defer func() { _ = os.Remove("config.yaml") }()

	cfg, err := config.Load("config")
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Environment variables should override config file
	assert.Equal(t, "8888", cfg.App.Port)              // from env
	assert.Equal(t, "sqlite", cfg.Db.Driver)           // from env
	assert.Equal(t, "production", cfg.App.Environment) // from config file
	assert.Equal(t, "db.example.com", cfg.Db.Host)     // from config file
}

func TestDSNPostgreSQL(t *testing.T) {
	db := &config.DBConfig{
		Driver:   "postgres",
		Host:     "localhost",
		Port:     "5432",
		User:     "testuser",
		Password: "testpass",
		Name:     "testdb",
		SSLMode:  "disable",
	}

	expected := "host=localhost port=5432 user=testuser password=testpass dbname=testdb sslmode=disable"
	assert.Equal(t, expected, db.DSN())
}

func TestDSNSQLite(t *testing.T) {
	db := &config.DBConfig{
		Driver:   "sqlite",
		FilePath: "./test.db",
	}

	assert.Equal(t, "./test.db", db.DSN())
}

func TestDSNUnknownDriver(t *testing.T) {
	db := &config.DBConfig{
		Driver: "unknown",
	}

	assert.Equal(t, "", db.DSN())
}

func TestIsDevelopment(t *testing.T) {
	tests := []struct {
		env      string
		expected bool
	}{
		{"development", true},
		{"dev", true},
		{"production", false},
		{"prod", false},
		{"testing", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.env, func(t *testing.T) {
			cfg := &config.AppConfig{Environment: tt.env}
			assert.Equal(t, tt.expected, cfg.IsDevelopment())
		})
	}
}

func TestIsProduction(t *testing.T) {
	tests := []struct {
		env      string
		expected bool
	}{
		{"production", true},
		{"prod", true},
		{"development", false},
		{"dev", false},
		{"testing", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.env, func(t *testing.T) {
			cfg := &config.AppConfig{Environment: tt.env}
			assert.Equal(t, tt.expected, cfg.IsProduction())
		})
	}
}

func TestInvalidConfigFile(t *testing.T) {
	clearEnvVars(t)

	// Create invalid YAML file
	tempDir := t.TempDir()
	invalidConfigPath := tempDir + "/config.yaml"
	err := os.WriteFile(invalidConfigPath, []byte("invalid: yaml: content: ["), 0644)
	require.NoError(t, err)

	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()

	err = os.Chdir(tempDir)
	require.NoError(t, err)

	cfg, err := config.Load("config")
	assert.Nil(t, cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error reading config file")
}

func TestInvalidConfigStructure(t *testing.T) {
	clearEnvVars(t)

	// Change to testdata directory
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()

	err = os.Chdir("testdata")
	require.NoError(t, err)

	// Copy config-invalid-structure.yaml to config.yaml
	copyFile(t, "config-invalid-structure.yaml", "config.yaml")
	defer func() { _ = os.Remove("config.yaml") }()

	cfg, err := config.Load("config")
	assert.Nil(t, cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error unmarshaling config")
}

// Helper functions

func clearEnvVars(t *testing.T) {
	envVars := []string{
		"TAIGA_APP_PORT",
		"TAIGA_APP_ENVIRONMENT",
		"TAIGA_DB_DRIVER",
		"TAIGA_DB_HOST",
		"TAIGA_DB_PORT",
		"TAIGA_DB_USER",
		"TAIGA_DB_PASSWORD",
		"TAIGA_DB_NAME",
		"TAIGA_DB_SSLMODE",
		"TAIGA_DB_FILEPATH",
	}

	for _, env := range envVars {
		t.Setenv(env, "")
	}
}

func copyFile(t *testing.T, src, dst string) {
	data, err := os.ReadFile(src)
	require.NoError(t, err)
	err = os.WriteFile(dst, data, 0644)
	require.NoError(t, err)
}
