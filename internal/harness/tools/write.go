package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WriteTool creates or overwrites files under the project root. When
// overwriting, the previous contents are returned in the tool output
// as a pre-write snapshot (design D7). A successful write marks the
// file in the FileTracker so Edit can modify it later without a
// separate Read.
type WriteTool struct {
	projectRoot string
	tracker     *FileTracker
}

// NewWriteTool constructs a Write tool. projectRoot is the absolute
// directory that bounds the writable file-system scope; writes outside
// the root are rejected with an error.
func NewWriteTool(projectRoot string, tracker *FileTracker) *WriteTool {
	return &WriteTool{projectRoot: projectRoot, tracker: tracker}
}

// ToolNameWrite is the canonical registered name.
const ToolNameWrite = "Write"

func (w *WriteTool) Name() string        { return ToolNameWrite }
func (w *WriteTool) Description() string { return "Write a file (creates or overwrites)" }
func (w *WriteTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "file_path": {"type":"string","description":"Absolute or project-relative path to the file"},
    "content":   {"type":"string","description":"Content to write"}
  },
  "required": ["file_path","content"]
}`)
}
func (w *WriteTool) OutputEnvelope() string { return "text" }
func (w *WriteTool) RiskLevel() RiskLevel   { return RiskWrite }

type writeArgs struct {
	FilePath *string `json:"file_path"`
	Content  *string `json:"content"`
}

func (w *WriteTool) Execute(_ context.Context, input json.RawMessage) (*ToolExecResult, error) {
	var in writeArgs
	if err := json.Unmarshal(input, &in); err != nil {
		return ErrorResult("invalid input: %v", err), nil
	}
	if in.FilePath == nil || *in.FilePath == "" {
		return ErrorResult("file_path is required"), nil
	}
	if in.Content == nil {
		return ErrorResult("content is required"), nil
	}

	abs, err := resolveInRoot(w.projectRoot, *in.FilePath)
	if err != nil {
		return ErrorResult("%v", err), nil
	}

	var snapshot []byte
	if existing, err := os.ReadFile(abs); err == nil {
		snapshot = existing
	} else if !os.IsNotExist(err) {
		return ErrorResult("read existing: %v", err), nil
	}

	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return ErrorResult("mkdir parent: %v", err), nil
	}
	if err := os.WriteFile(abs, []byte(*in.Content), 0o644); err != nil {
		return ErrorResult("write: %v", err), nil
	}

	w.tracker.MarkRead(abs)

	var b strings.Builder
	fmt.Fprintf(&b, "Wrote %d bytes to %s", len(*in.Content), abs)
	if snapshot != nil {
		fmt.Fprintf(&b, " (overwrote %d bytes).\n\nPrevious content:\n%s", len(snapshot), snapshot)
	} else {
		b.WriteString(" (new file).")
	}
	return SuccessResult(b.String(), "text"), nil
}

// resolveInRoot canonicalizes a user-supplied path. Relative paths
// resolve under root. The resulting absolute path must sit inside
// root — anything escaping via ".." or an absolute path outside root
// is rejected so the model can't punch through the harness's
// file-system scope.
//
// Symlink handling (WU-094 C-4): `filepath.Clean` + `filepath.Rel`
// alone don't catch symlinks — a path like `./secret → /etc/passwd`
// lives textually under the root but resolves elsewhere. After the
// textual check, walk the path with `os.Lstat` on each component
// and reject any symlink whose target escapes the root. For paths
// that don't yet exist (Write creating a new file), evaluate the
// deepest existing ancestor — creating *within* an evaluated-safe
// directory is allowed; creating *through* a symlink that escapes
// is not.
func resolveInRoot(root, path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("file_path is empty")
	}
	candidate := path
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(root, candidate)
	}
	candidate = filepath.Clean(candidate)
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve root: %w", err)
	}
	absRoot = filepath.Clean(absRoot)

	// Textual scope check.
	if rel, err := filepath.Rel(absRoot, candidate); err != nil ||
		rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path outside project root: %s", path)
	}

	// Walk toward the deepest existing ancestor and EvalSymlinks
	// there. If the full path exists, evaluate it directly. The
	// result must still sit under an evaluated root (symlinked
	// project roots are fine; symlinked descendants pointing out
	// of the evaluated root are not).
	evalRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		// Root itself must resolve — otherwise scope is undefined.
		return "", fmt.Errorf("resolve project root: %w", err)
	}
	evalRoot = filepath.Clean(evalRoot)

	check := candidate
	for {
		resolved, err := filepath.EvalSymlinks(check)
		if err == nil {
			resolved = filepath.Clean(resolved)
			if rel, relErr := filepath.Rel(evalRoot, resolved); relErr != nil ||
				rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return "", fmt.Errorf("path escapes project root via symlink: %s", path)
			}
			// Check passed. Return the pre-eval candidate so tools
			// see the requested path (creation follows symlinked
			// parents that resolve inside the root, which is fine).
			return candidate, nil
		}
		// Path doesn't exist yet (common on Write / Edit creating
		// a new file). Strip the deepest component and try again;
		// if we hit the root, the full chain is within scope.
		parent := filepath.Dir(check)
		if parent == check {
			return candidate, nil
		}
		if parent == absRoot || parent == evalRoot {
			return candidate, nil
		}
		check = parent
	}
}
