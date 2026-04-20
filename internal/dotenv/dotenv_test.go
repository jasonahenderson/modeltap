package dotenv

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// unsetEnv removes a variable from the process environment and
// restores the prior state on test cleanup. t.Setenv(key, "") is
// not a substitute — it leaves the variable "set to empty", which
// godotenv.Load interprets as "already set, do not touch."
func unsetEnv(t *testing.T, key string) {
	t.Helper()
	prev, had := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unsetenv %s: %v", key, err)
	}
	t.Cleanup(func() {
		if had {
			os.Setenv(key, prev)
		} else {
			os.Unsetenv(key)
		}
	})
}

// withCwd switches into dir for the duration of the test so the
// ./.env resolution path can be exercised without polluting the
// package's real working directory.
func withCwd(t *testing.T, dir string) {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(prev)
	})
}

// TestLoad_Missing: no .env anywhere is a silent success.
func TestLoad_Missing(t *testing.T) {
	dir := t.TempDir()
	withCwd(t, dir)
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("MODELTAP_DOTENV", "")
	t.Setenv("MODELTAP_DEBUG_DOTENV", "")

	var buf bytes.Buffer
	if err := Load(&buf); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("no files should produce no stderr output, got %q", buf.String())
	}
}

// TestLoad_ProjectLocal: a ./.env file populates os.Getenv.
func TestLoad_ProjectLocal(t *testing.T) {
	dir := t.TempDir()
	withCwd(t, dir)
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", "")
	unsetEnv(t, "PATCH7_LOCAL_KEY")
	if err := os.WriteFile(".env", []byte("PATCH7_LOCAL_KEY=from-local\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := Load(nil); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := os.Getenv("PATCH7_LOCAL_KEY"); got != "from-local" {
		t.Errorf("PATCH7_LOCAL_KEY = %q, want from-local", got)
	}
}

// TestLoad_ProcessEnvWins: a variable already set in the process env
// is NOT overwritten by .env — matches Node/Python dotenv behavior.
func TestLoad_ProcessEnvWins(t *testing.T) {
	dir := t.TempDir()
	withCwd(t, dir)
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("PATCH7_PRESET", "from-shell")
	if err := os.WriteFile(".env", []byte("PATCH7_PRESET=from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := Load(nil); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := os.Getenv("PATCH7_PRESET"); got != "from-shell" {
		t.Errorf("PATCH7_PRESET = %q, want from-shell (process env should win)", got)
	}
}

// TestLoad_ProjectBeatsUser: ./.env wins over ~/.modeltap/.env when
// both set the same key.
func TestLoad_ProjectBeatsUser(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	withCwd(t, cwd)
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	unsetEnv(t, "PATCH7_PRIORITY")

	userDir := filepath.Join(home, ".modeltap")
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userDir, ".env"), []byte("PATCH7_PRIORITY=user\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cwd, ".env"), []byte("PATCH7_PRIORITY=project\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := Load(nil); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := os.Getenv("PATCH7_PRIORITY"); got != "project" {
		t.Errorf("PATCH7_PRIORITY = %q, want project", got)
	}
}

// TestLoad_UserLevelOnly: with no project-local file, user-level is
// still picked up.
func TestLoad_UserLevelOnly(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	withCwd(t, cwd)
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	unsetEnv(t, "PATCH7_USER_ONLY")

	userDir := filepath.Join(home, ".modeltap")
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userDir, ".env"), []byte("PATCH7_USER_ONLY=user-wins\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := Load(nil); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := os.Getenv("PATCH7_USER_ONLY"); got != "user-wins" {
		t.Errorf("PATCH7_USER_ONLY = %q, want user-wins", got)
	}
}

// TestLoad_XDGConfigHome: $XDG_CONFIG_HOME/modeltap/.env works when
// the env var is set — mirrors PATCH-0006's config-dir behavior.
func TestLoad_XDGConfigHome(t *testing.T) {
	xdg := t.TempDir()
	home := t.TempDir()
	cwd := t.TempDir()
	withCwd(t, cwd)
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", xdg)
	unsetEnv(t, "PATCH7_XDG_KEY")

	xdgDir := filepath.Join(xdg, "modeltap")
	if err := os.MkdirAll(xdgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(xdgDir, ".env"), []byte("PATCH7_XDG_KEY=xdg\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := Load(nil); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := os.Getenv("PATCH7_XDG_KEY"); got != "xdg" {
		t.Errorf("PATCH7_XDG_KEY = %q, want xdg", got)
	}
}

// TestLoad_Malformed: a syntactically broken .env returns an error.
// The user paid the cost to write the file, so a silent failure
// would be confusing.
func TestLoad_Malformed(t *testing.T) {
	dir := t.TempDir()
	withCwd(t, dir)
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", "")
	// godotenv rejects lines lacking `=` that aren't comments.
	if err := os.WriteFile(".env", []byte("this line has no equals sign\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := Load(nil); err == nil {
		t.Fatal("expected error for malformed .env")
	}
}

// TestLoad_Disabled: MODELTAP_DOTENV=false skips loading entirely
// even if a .env file exists.
func TestLoad_Disabled(t *testing.T) {
	dir := t.TempDir()
	withCwd(t, dir)
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("MODELTAP_DOTENV", "false")
	unsetEnv(t, "PATCH7_SKIP_ME")
	if err := os.WriteFile(".env", []byte("PATCH7_SKIP_ME=should-not-load\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := Load(nil); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := os.Getenv("PATCH7_SKIP_ME"); got != "" {
		t.Errorf("PATCH7_SKIP_ME = %q, want empty (loader disabled)", got)
	}
}

// TestLoad_DebugOutput: MODELTAP_DEBUG_DOTENV=1 triggers a one-line
// stderr trace naming the loaded files. Silent by default.
func TestLoad_DebugOutput(t *testing.T) {
	dir := t.TempDir()
	withCwd(t, dir)
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("MODELTAP_DEBUG_DOTENV", "1")
	if err := os.WriteFile(".env", []byte("PATCH7_DEBUG_KEY=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := Load(&buf); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("expected stderr trace when MODELTAP_DEBUG_DOTENV=1")
	}
}

func TestDisabledVariants(t *testing.T) {
	for _, v := range []string{"false", "0", "no", "off"} {
		t.Run(v, func(t *testing.T) {
			t.Setenv("MODELTAP_DOTENV", v)
			if !disabled() {
				t.Errorf("MODELTAP_DOTENV=%q should disable loader", v)
			}
		})
	}
	t.Run("unset", func(t *testing.T) {
		t.Setenv("MODELTAP_DOTENV", "")
		if disabled() {
			t.Error("unset MODELTAP_DOTENV should not disable loader")
		}
	})
	t.Run("true", func(t *testing.T) {
		t.Setenv("MODELTAP_DOTENV", "true")
		if disabled() {
			t.Error("MODELTAP_DOTENV=true should not disable loader")
		}
	})
}
