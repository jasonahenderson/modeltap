// Package dotenv loads variables from a .env file into the process
// environment at startup. It is a thin wrapper around joho/godotenv
// that encodes modeltap's file-resolution order (PATCH-0007):
//
//  1. Existing process environment is never overridden.
//  2. ./.env (project-local, relative to the current working directory)
//     wins over user-level files.
//  3. $HOME/.modeltap/.env and, when $XDG_CONFIG_HOME is set,
//     $XDG_CONFIG_HOME/modeltap/.env are loaded as user-level defaults.
//
// Missing files are silent (the common case for users not using
// dotenv). A .env file that exists but fails to parse returns an
// error.
package dotenv

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

// Load resolves the list of candidate .env files (in precedence
// order) and loads any that exist. stderr receives a one-line
// "loaded: <paths>" note only when MODELTAP_DEBUG_DOTENV=1 is set;
// otherwise the loader is silent on success.
//
// Set MODELTAP_DOTENV=false (or 0 / no / off) to skip loading
// entirely — useful in CI, containers, or when debugging which
// values are coming from where.
func Load(stderr io.Writer) error {
	if disabled() {
		return nil
	}
	paths := candidates()
	if len(paths) == 0 {
		return nil
	}
	if err := godotenv.Load(paths...); err != nil {
		return fmt.Errorf("dotenv: %w", err)
	}
	if stderr != nil && debugEnabled() {
		fmt.Fprintf(stderr, "dotenv: loaded %v\n", paths)
	}
	return nil
}

// candidates returns the .env files that exist, in godotenv precedence
// order — earlier entries win ties because godotenv.Load skips vars
// already set by prior files.
func candidates() []string {
	var paths []string
	if _, err := os.Stat(".env"); err == nil {
		paths = append(paths, ".env")
	}
	seen := map[string]struct{}{}
	for _, p := range userLevelPaths() {
		if p == "" {
			continue
		}
		if _, dup := seen[p]; dup {
			continue
		}
		seen[p] = struct{}{}
		if _, err := os.Stat(p); err == nil {
			paths = append(paths, p)
		}
	}
	return paths
}

// userLevelPaths returns the ordered list of user-level .env
// locations. $XDG_CONFIG_HOME/modeltap/.env wins over ~/.modeltap/.env
// when both are set, mirroring PATCH-0006's config-dir behavior.
func userLevelPaths() []string {
	var out []string
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		out = append(out, filepath.Join(xdg, "modeltap", ".env"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		out = append(out, filepath.Join(home, ".modeltap", ".env"))
	}
	return out
}

func disabled() bool {
	switch os.Getenv("MODELTAP_DOTENV") {
	case "false", "0", "no", "off":
		return true
	}
	return false
}

func debugEnabled() bool {
	return os.Getenv("MODELTAP_DEBUG_DOTENV") == "1"
}
