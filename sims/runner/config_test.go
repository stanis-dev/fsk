package main

import (
	"path/filepath"
	"testing"
)

func TestReadEnvToken(t *testing.T) {
	dir := t.TempDir()
	env := filepath.Join(dir, ".env")
	writeFile(t, env, "SOMETHING=else\nCLAUDE_CODE_OAUTH_TOKEN=\"sk-abc123\"\nMORE=1\n")

	tok, err := readEnvToken(env)
	if err != nil {
		t.Fatalf("readEnvToken: %v", err)
	}
	if tok != "sk-abc123" {
		t.Errorf("token = %q, want sk-abc123", tok)
	}
}

func TestReadEnvToken_Missing(t *testing.T) {
	dir := t.TempDir()
	env := filepath.Join(dir, ".env")
	writeFile(t, env, "NOTHING=here\n")
	if _, err := readEnvToken(env); err == nil {
		t.Fatal("expected error when token key is absent")
	}
}

func TestReadEnvToken_Empty(t *testing.T) {
	dir := t.TempDir()
	env := filepath.Join(dir, ".env")
	writeFile(t, env, "CLAUDE_CODE_OAUTH_TOKEN=\n")
	if _, err := readEnvToken(env); err == nil {
		t.Fatal("expected error when token value is empty")
	}
}
