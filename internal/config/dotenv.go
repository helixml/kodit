package config

import (
	"os"

	"github.com/joho/godotenv"
)

// LoadDotEnv loads environment variables from a .env file.
// If path is empty, it loads from ".env" in the current directory.
// If the file does not exist, it silently returns nil (not an error).
func LoadDotEnv(path string) error {
	if path == "" {
		path = ".env"
	}

	// Check if file exists first
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}

	return godotenv.Load(path)
}

// MustLoadDotEnv loads environment variables from a .env file.
// Unlike LoadDotEnv, it returns an error if the file does not exist.
func MustLoadDotEnv(path string) error {
	if path == "" {
		path = ".env"
	}
	return godotenv.Load(path)
}

// LoadDotEnvFromFiles loads environment variables from multiple .env files.
// Files are processed in order. Note: godotenv.Load does NOT override existing
// environment variables - the first file that sets a variable wins.
// Non-existent files are silently skipped.
func LoadDotEnvFromFiles(paths ...string) error {
	for _, path := range paths {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		}
		if err := godotenv.Load(path); err != nil {
			return err
		}
	}
	return nil
}

// OverloadDotEnvFromFiles loads environment variables from multiple .env files,
// overwriting any existing values. Files are processed in order, with later
// files overwriting earlier values. Non-existent files are silently skipped.
func OverloadDotEnvFromFiles(paths ...string) error {
	for _, path := range paths {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		}
		if err := godotenv.Overload(path); err != nil {
			return err
		}
	}
	return nil
}

// LoadConfig loads configuration from a .env file (optional) and environment variables.
// The .env file is loaded first if it exists, then environment variables override.
// This matches Python's pydantic-settings behavior.
func LoadConfig(envPath string) (AppConfig, error) {
	// Load .env file if specified (silently skip if not found)
	if err := LoadDotEnv(envPath); err != nil {
		return AppConfig{}, err
	}

	// Load from environment variables
	envCfg, err := LoadFromEnv()
	if err != nil {
		return AppConfig{}, err
	}

	return envCfg.Normalize().ToAppConfig(), nil
}
