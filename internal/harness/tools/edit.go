package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// EditTool modifies an existing file via exact-string replacement.
// The Read-before-mutate contract is enforced via FileTracker: the
// file must have been read (or written) earlier in the session before
// Edit will operate on it (design D5 / D8).
type EditTool struct {
	projectRoot string
	tracker     *FileTracker
}

// NewEditTool constructs an Edit tool bound to a project root and a
// shared FileTracker (typically the Executor's tracker).
func NewEditTool(projectRoot string, tracker *FileTracker) *EditTool {
	return &EditTool{projectRoot: projectRoot, tracker: tracker}
}

// ToolNameEdit is the canonical registered name.
const ToolNameEdit = "Edit"

func (e *EditTool) Name() string        { return ToolNameEdit }
func (e *EditTool) Description() string { return "Replace exact-match text in a file" }
func (e *EditTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "file_path":   {"type":"string","description":"Absolute or project-relative path to the file"},
    "old_string":  {"type":"string","description":"Exact text to replace"},
    "new_string":  {"type":"string","description":"Replacement text"},
    "replace_all": {"type":"boolean","description":"Replace all occurrences (default false)"}
  },
  "required": ["file_path","old_string","new_string"]
}`)
}
func (e *EditTool) OutputEnvelope() string { return "text" }
func (e *EditTool) RiskLevel() RiskLevel   { return RiskWrite }

type editArgs struct {
	FilePath   *string `json:"file_path"`
	OldString  *string `json:"old_string"`
	NewString  *string `json:"new_string"`
	ReplaceAll bool    `json:"replace_all"`
}

func (e *EditTool) Execute(_ context.Context, input json.RawMessage) (*ToolExecResult, error) {
	var in editArgs
	if err := json.Unmarshal(input, &in); err != nil {
		return ErrorResult("invalid input: %v", err), nil
	}
	if in.FilePath == nil || *in.FilePath == "" {
		return ErrorResult("file_path is required"), nil
	}
	if in.OldString == nil {
		return ErrorResult("old_string is required"), nil
	}
	if in.NewString == nil {
		return ErrorResult("new_string is required"), nil
	}
	if *in.OldString == "" {
		return ErrorResult("old_string cannot be empty"), nil
	}

	abs, err := resolveInRoot(e.projectRoot, *in.FilePath)
	if err != nil {
		return ErrorResult("%v", err), nil
	}

	if !e.tracker.HasRead(abs) {
		return ErrorResult("Edit requires a prior Read of %s; call Read before Edit", abs), nil
	}

	contents, err := os.ReadFile(abs)
	if err != nil {
		return ErrorResult("read %s: %v", abs, err), nil
	}
	body := string(contents)

	count := strings.Count(body, *in.OldString)
	switch {
	case count == 0:
		return ErrorResult("old_string not found in %s", abs), nil
	case count > 1 && !in.ReplaceAll:
		return ErrorResult("old_string is not unique (%d occurrences) in %s; use replace_all or provide more context", count, abs), nil
	}

	var updated string
	var replaced int
	if in.ReplaceAll {
		updated = strings.ReplaceAll(body, *in.OldString, *in.NewString)
		replaced = count
	} else {
		updated = strings.Replace(body, *in.OldString, *in.NewString, 1)
		replaced = 1
	}

	if err := os.WriteFile(abs, []byte(updated), 0o644); err != nil {
		return ErrorResult("write %s: %v", abs, err), nil
	}

	noun := "occurrence"
	if replaced != 1 {
		noun = "occurrences"
	}
	out := fmt.Sprintf("Edited %s — replaced %d %s.\n\n- %s\n+ %s",
		abs, replaced, noun, truncateForDisplay(*in.OldString), truncateForDisplay(*in.NewString))
	return SuccessResult(out, "text"), nil
}

// truncateForDisplay shortens very long strings so the diff line stays
// readable in the conversation transcript. 200 chars is enough to
// spot the change without flooding the output.
func truncateForDisplay(s string) string {
	const limit = 200
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "…"
}
