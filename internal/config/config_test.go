package config_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitea.deepak.science/deepak/trygo/internal/config"
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

	cfg, err := config.Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Test default values
	assert.Equal(t, "8080", cfg.Port)
	assert.Equal(t, "development", cfg.Environment)
	assert.Equal(t, "sqlite", cfg.Database.Driver)
	assert.Equal(t, "localhost", cfg.Database.Host)
	assert.Equal(t, "5432", cfg.Database.Port)
	assert.Equal(t, "trygo", cfg.Database.Name)
	assert.Equal(t, "disable", cfg.Database.SSLMode)
	assert.Equal(t, "./data.db", cfg.Database.FilePath)
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

	cfg, err := config.Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Test loaded values
	assert.Equal(t, "9090", cfg.Port)
	assert.Equal(t, "production", cfg.Environment)
	assert.Equal(t, "postgres", cfg.Database.Driver)
	assert.Equal(t, "db.example.com", cfg.Database.Host)
	assert.Equal(t, "5433", cfg.Database.Port)
	assert.Equal(t, "testuser", cfg.Database.User)
	assert.Equal(t, "testpass", cfg.Database.Password)
	assert.Equal(t, "testdb", cfg.Database.Name)
	assert.Equal(t, "require", cfg.Database.SSLMode)
	assert.Equal(t, "/custom/path.db", cfg.Database.FilePath)
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

	cfg, err := config.Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Test partial config with defaults
	assert.Equal(t, "3000", cfg.Port)
	assert.Equal(t, "development", cfg.Environment) // default
	assert.Equal(t, "sqlite", cfg.Database.Driver)
	assert.Equal(t, "localhost", cfg.Database.Host) // default
	assert.Equal(t, "5432", cfg.Database.Port)      // default
	assert.Equal(t, "./test.db", cfg.Database.FilePath)
}

func TestEnvironmentVariables(t *testing.T) {
	clearEnvVars(t)

	// Set environment variables
	t.Setenv("TRYGO_PORT", "7777")
	t.Setenv("TRYGO_ENVIRONMENT", "testing")
	t.Setenv("TRYGO_DATABASE_DRIVER", "postgres")
	t.Setenv("TRYGO_DATABASE_HOST", "env-db-host")
	t.Setenv("TRYGO_DATABASE_USER", "env-user")

	// Change to a directory without config files
	tempDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()

	err = os.Chdir(tempDir)
	require.NoError(t, err)

	cfg, err := config.Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Environment variables should override defaults
	assert.Equal(t, "7777", cfg.Port)
	assert.Equal(t, "testing", cfg.Environment)
	assert.Equal(t, "postgres", cfg.Database.Driver)
	assert.Equal(t, "env-db-host", cfg.Database.Host)
	assert.Equal(t, "env-user", cfg.Database.User)
	// Defaults should still apply for unset env vars
	assert.Equal(t, "5432", cfg.Database.Port)
	assert.Equal(t, "trygo", cfg.Database.Name)
}

func TestEnvironmentVariablesOverrideConfig(t *testing.T) {
	clearEnvVars(t)

	// Set environment variables that should override config file
	t.Setenv("TRYGO_PORT", "8888")
	t.Setenv("TRYGO_DATABASE_DRIVER", "sqlite")

	// Change to testdata directory
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()

	err = os.Chdir("testdata")
	require.NoError(t, err)

	// Copy config-full.yaml to config.yaml
	copyFile(t, "config-full.yaml", "config.yaml")
	defer func() { _ = os.Remove("config.yaml") }()

	cfg, err := config.Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Environment variables should override config file
	assert.Equal(t, "8888", cfg.Port)                    // from env
	assert.Equal(t, "sqlite", cfg.Database.Driver)       // from env
	assert.Equal(t, "production", cfg.Environment)       // from config file
	assert.Equal(t, "db.example.com", cfg.Database.Host) // from config file
}

func TestDSNPostgreSQL(t *testing.T) {
	db := &config.DB{
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
	db := &config.DB{
		Driver:   "sqlite",
		FilePath: "./test.db",
	}

	assert.Equal(t, "./test.db", db.DSN())
}

func TestDSNUnknownDriver(t *testing.T) {
	db := &config.DB{
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
			cfg := &config.Config{Environment: tt.env}
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
			cfg := &config.Config{Environment: tt.env}
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

	cfg, err := config.Load()
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

	cfg, err := config.Load()
	assert.Nil(t, cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error unmarshaling config")
}

// Helper functions

func clearEnvVars(t *testing.T) {
	envVars := []string{
		"TRYGO_PORT",
		"TRYGO_ENVIRONMENT",
		"TRYGO_DATABASE_DRIVER",
		"TRYGO_DATABASE_HOST",
		"TRYGO_DATABASE_PORT",
		"TRYGO_DATABASE_USER",
		"TRYGO_DATABASE_PASSWORD",
		"TRYGO_DATABASE_NAME",
		"TRYGO_DATABASE_SSLMODE",
		"TRYGO_DATABASE_FILEPATH",
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
