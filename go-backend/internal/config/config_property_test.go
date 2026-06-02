package config

import (
	"os"
	"testing"

	"pgregory.net/rapid"
)

// Property 29: Config environment variable loading (env vars override .env)
func TestProperty_ConfigEnvVarPriority(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		envValue := rapid.StringMatching(`[a-z]{5,20}`).Draw(t, "envValue")

		// Set env var
		os.Setenv("TES_JWT_SECRET", "a-very-long-secret-key-for-testing-purposes-1234567890")
		os.Setenv("TES_LLM_KEY_MASTER", "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY=") // 32 bytes base64
		os.Setenv("TES_UPLOAD_ROOT", envValue)
		defer func() {
			os.Unsetenv("TES_JWT_SECRET")
			os.Unsetenv("TES_LLM_KEY_MASTER")
			os.Unsetenv("TES_UPLOAD_ROOT")
		}()

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load failed: %v", err)
		}

		if cfg.UploadRoot != envValue {
			t.Fatalf("env var should override default: got %q, want %q", cfg.UploadRoot, envValue)
		}
	})
}
