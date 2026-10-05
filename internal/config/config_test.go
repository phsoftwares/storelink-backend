package config

import (
	"os"
	"path/filepath"
	"testing"
)

func setRequiredEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://storelink:storelink@127.0.0.1:5432/storelink?sslmode=disable")
	t.Setenv("JWT_SECRET", "integration-only-jwt-secret-with-at-least-32-bytes")
	t.Setenv("ADMIN_EMAIL", "")
	t.Setenv("ADMIN_NAME", "")
	t.Setenv("ADMIN_PASSWORD", "")
}

func TestLoadDoesNotRequireSharedAgentAPIKey(t *testing.T) {
	setRequiredEnvironment(t)
	_, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
}

func TestAdminSeedConfigurationMustBeComplete(t *testing.T) {
	setRequiredEnvironment(t)
	t.Setenv("ADMIN_EMAIL", "admin@example.test")
	if _, err := Load(); err == nil {
		t.Fatal("Load() should reject partial initial administrator settings")
	}

	t.Setenv("ADMIN_NAME", "StoreLink Administrator")
	t.Setenv("ADMIN_PASSWORD", "test-password-longer-than-12")
	if _, err := Load(); err != nil {
		t.Fatalf("Load() rejected complete initial administrator settings: %v", err)
	}
}

func TestLoadDotEnvReadsCurrentDirectoryWithoutOverridingProcessEnvironment(t *testing.T) {
	const fromFile = "from-dotenv-file"
	const fromProcess = "from-process-environment"
	keyFromFile := "STORELINK_DOTENV_TEST_FILE"
	keyFromProcess := "STORELINK_DOTENV_TEST_PROCESS"

	oldDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	temporaryDirectory := t.TempDir()
	if err := os.WriteFile(filepath.Join(temporaryDirectory, ".env"), []byte(
		keyFromFile+"="+fromFile+"\n"+keyFromProcess+"="+fromFile+"\n"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.Chdir(temporaryDirectory); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldDirectory)
		_ = os.Unsetenv(keyFromFile)
		_ = os.Unsetenv(keyFromProcess)
	})
	if err := os.Setenv(keyFromProcess, fromProcess); err != nil {
		t.Fatalf("Setenv() error = %v", err)
	}
	if err := loadDotEnv(); err != nil {
		t.Fatalf("loadDotEnv() error = %v", err)
	}
	if got := os.Getenv(keyFromFile); got != fromFile {
		t.Fatalf("file value = %q, want %q", got, fromFile)
	}
	if got := os.Getenv(keyFromProcess); got != fromProcess {
		t.Fatalf("process value = %q, want %q", got, fromProcess)
	}
}
