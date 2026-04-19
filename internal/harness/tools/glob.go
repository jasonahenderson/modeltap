package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// GlobTool finds files matching a glob pattern under a project-rooted
// directory. Supports `**` recursive wildcards via doublestar/v4.
// Results are returned relative to the project root, sorted by
// modification time (newest first).
type GlobTool struct {
	projectRoot string
}

// NewGlobTool constructs a Glob tool bound to projectRoot.
func NewGlobTool(projectRoot string) *GlobTool {
	return &GlobTool{projectRoot: projectRoot}
}

// ToolNameGlob is the canonical registered name.
const ToolNameGlob = "Glob"

func (g *GlobTool) Name() string        { return ToolNameGlob }
func (g *GlobTool) Description() string { return "Find files by glob pattern (supports **)" }
func (g *GlobTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "pattern": {"type":"string","description":"Glob pattern (supports **)"},
    "path":    {"type":"string","description":"Directory to search (optional, defaults to project root)"}
  },
  "required": ["pattern"]
}`)
}
func (g *GlobTool) OutputEnvelope() string { return "text" }
func (g *GlobTool) RiskLevel() RiskLevel   { return RiskReadOnly }

type globArgs struct {
	Pattern *string `json:"pattern"`
	Path    string  `json:"path"`
}

func (g *GlobTool) Execute(_ context.Context, input json.RawMessage) (*ToolExecResult, error) {
	var in globArgs
	if err := json.Unmarshal(input, &in); err != nil {
		return ErrorResult("invalid input: %v", err), nil
	}
	if in.Pattern == nil || *in.Pattern == "" {
		return ErrorResult("pattern is required"), nil
	}

	searchDir := g.projectRoot
	if in.Path != "" {
		abs, err := resolveInRoot(g.projectRoot, in.Path)
		if err != nil {
			return ErrorResult("%v", err), nil
		}
		searchDir = abs
	}

	fsys := os.DirFS(searchDir)
	matches, err := doublestar.Glob(fsys, *in.Pattern, doublestar.WithFilesOnly())
	if err != nil {
		return ErrorResult("glob: %v", err), nil
	}

	type entry struct {
		rel   string
		mtime int64
	}
	entries := make([]entry, 0, len(matches))
	for _, m := range matches {
		abs := filepath.Join(searchDir, m)
		info, err := os.Stat(abs)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(g.projectRoot, abs)
		if err != nil {
			rel = m
		}
		entries = append(entries, entry{rel: rel, mtime: info.ModTime().UnixNano()})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].mtime > entries[j].mtime
	})

	if len(entries) == 0 {
		return SuccessResult("no matches", "text"), nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%d match(es):\n", len(entries))
	for _, e := range entries {
		b.WriteString(e.rel)
		b.WriteByte('\n')
	}
	return SuccessResult(strings.TrimRight(b.String(), "\n"), "text"), nil
}
