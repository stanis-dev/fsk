package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeEnv(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

func TestReadEnvToken(t *testing.T) {
	env := filepath.Join(t.TempDir(), ".env")
	writeEnv(t, env, "SOMETHING=else\nCLAUDE_CODE_OAUTH_TOKEN=\"sk-abc123\"\nMORE=1\n")

	tok, err := readEnvToken(env)
	if err != nil {
		t.Fatalf("readEnvToken: %v", err)
	}
	if tok != "sk-abc123" {
		t.Errorf("token = %q, want sk-abc123", tok)
	}
}

func TestReadEnvToken_Missing(t *testing.T) {
	env := filepath.Join(t.TempDir(), ".env")
	writeEnv(t, env, "NOTHING=here\n")
	if _, err := readEnvToken(env); err == nil {
		t.Fatal("expected error when token key is absent")
	}
}

func TestReadEnvToken_Empty(t *testing.T) {
	env := filepath.Join(t.TempDir(), ".env")
	writeEnv(t, env, "CLAUDE_CODE_OAUTH_TOKEN=\n")
	if _, err := readEnvToken(env); err == nil {
		t.Fatal("expected error when token value is empty")
	}
}

func TestResolveRunsDir_EnvOverride(t *testing.T) {
	want := t.TempDir()
	t.Setenv("FISKALY_RUNS_DIR", want)
	got, err := resolveRunsDir()
	if err != nil {
		t.Fatalf("resolveRunsDir: %v", err)
	}
	if got != want {
		t.Errorf("runs dir = %q, want %q", got, want)
	}
}

func TestResolveRunsDir_Default(t *testing.T) {
	t.Setenv("FISKALY_RUNS_DIR", "")
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}
	got, err := resolveRunsDir()
	if err != nil {
		t.Fatalf("resolveRunsDir: %v", err)
	}
	if want := filepath.Join(home, ".cache", "fiskaly-eval"); got != want {
		t.Errorf("runs dir = %q, want %q", got, want)
	}
}

func TestResolveCORSOrigin_EnvOverride(t *testing.T) {
	t.Setenv("CORS_ORIGIN", "http://localhost:8081")
	if got := resolveCORSOrigin(); got != "http://localhost:8081" {
		t.Errorf("cors origin = %q, want http://localhost:8081", got)
	}
}

func TestResolveCORSOrigin_Default(t *testing.T) {
	t.Setenv("CORS_ORIGIN", "")
	if got := resolveCORSOrigin(); got != corsOrigin {
		t.Errorf("cors origin = %q, want %q", got, corsOrigin)
	}
}
