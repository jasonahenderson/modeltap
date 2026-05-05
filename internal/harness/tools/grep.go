package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// GrepTool searches file contents by regular expression. Walks a
// directory tree (optionally filtered by glob), reads each file once,
// and formats matches per `output_mode`. All paths in the output are
// reported relative to the project root.
type GrepTool struct {
	projectRoot string
	maxMatches  int // per-file cap to keep output bounded
}

// NewGrepTool constructs a Grep tool bound to projectRoot.
func NewGrepTool(projectRoot string) *GrepTool {
	return &GrepTool{projectRoot: projectRoot, maxMatches: 250}
}

// SetMaxMatches overrides the per-file match cap (tests use this).
func (g *GrepTool) SetMaxMatches(n int) { g.maxMatches = n }

// ToolNameGrep is the canonical registered name.
const ToolNameGrep = "Grep"

func (g *GrepTool) Name() string        { return ToolNameGrep }
func (g *GrepTool) Description() string { return "Search file contents by regex" }
func (g *GrepTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "pattern":          {"type":"string","description":"Regular expression pattern"},
    "path":             {"type":"string","description":"File or directory to search (optional)"},
    "glob":             {"type":"string","description":"Glob pattern to filter files"},
    "output_mode":      {"type":"string","enum":["content","files_with_matches","count"]},
    "context":          {"type":"integer","description":"Lines of context around matches"},
    "case_insensitive": {"type":"boolean","description":"Case-insensitive search"}
  },
  "required": ["pattern"]
}`)
}
func (g *GrepTool) OutputEnvelope() string { return "text" }
func (g *GrepTool) RiskLevel() RiskLevel   { return RiskReadOnly }

type grepArgs struct {
	Pattern         *string `json:"pattern"`
	Path            string  `json:"path"`
	Glob            string  `json:"glob"`
	OutputMode      string  `json:"output_mode"`
	Context         int     `json:"context"`
	CaseInsensitive bool    `json:"case_insensitive"`
}

const (
	grepModeFiles   = "files_with_matches"
	grepModeContent = "content"
	grepModeCount   = "count"
)

func (g *GrepTool) Execute(_ context.Context, input json.RawMessage) (*ToolExecResult, error) {
	var in grepArgs
	if err := json.Unmarshal(input, &in); err != nil {
		return ErrorResult("invalid input: %v", err), nil
	}
	if in.Pattern == nil || *in.Pattern == "" {
		return ErrorResult("pattern is required"), nil
	}

	mode := in.OutputMode
	if mode == "" {
		mode = grepModeFiles
	}
	switch mode {
	case grepModeFiles, grepModeContent, grepModeCount:
	default:
		return ErrorResult("invalid output_mode: %s", mode), nil
	}

	pat := *in.Pattern
	if in.CaseInsensitive {
		pat = "(?i)" + pat
	}
	re, err := regexp.Compile(pat)
	if err != nil {
		return ErrorResult("invalid regex: %v", err), nil
	}

	searchRoot := g.projectRoot
	if in.Path != "" {
		abs, err := resolveInRoot(g.projectRoot, in.Path)
		if err != nil {
			return ErrorResult("%v", err), nil
		}
		searchRoot = abs
	}

	type fileHits struct {
		relPath string
		lines   []grepLine
		count   int
	}
	var results []fileHits

	// Bound total bytes scanned to prevent a pathological repo
	// (10 GB log file, million-file tree) from OOMing the harness.
	// WU-094 H-15. Per-file grepFile already skips binary files and
	// caps matches; this is the aggregate backstop.
	var scannedBytes int64
	const maxScanBytes int64 = 256 * 1024 * 1024 // 256 MiB aggregate
	scanTruncated := false

	walkErr := filepath.WalkDir(searchRoot, func(path string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return nil // skip unreadable entries
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if in.Glob != "" {
			matched, mErr := filepath.Match(in.Glob, d.Name())
			if mErr != nil || !matched {
				return nil
			}
		}
		if info, ierr := d.Info(); ierr == nil {
			scannedBytes += info.Size()
			if scannedBytes > maxScanBytes {
				scanTruncated = true
				return filepath.SkipAll
			}
		}
		hits := grepFile(path, re, in.Context, g.maxMatches)
		if hits.count == 0 {
			return nil
		}
		rel, rErr := filepath.Rel(g.projectRoot, path)
		if rErr != nil {
			rel = path
		}
		results = append(results, fileHits{relPath: rel, lines: hits.lines, count: hits.count})
		return nil
	})
	_ = scanTruncated // reserved for a follow-up "partial results" banner
	if walkErr != nil {
		return ErrorResult("walk: %v", walkErr), nil
	}

	sort.Slice(results, func(i, j int) bool { return results[i].relPath < results[j].relPath })

	if len(results) == 0 {
		return SuccessResult("no matches", "text"), nil
	}

	var b strings.Builder
	switch mode {
	case grepModeFiles:
		for _, r := range results {
			b.WriteString(r.relPath)
			b.WriteByte('\n')
		}
	case grepModeCount:
		for _, r := range results {
			fmt.Fprintf(&b, "%s:%d\n", r.relPath, r.count)
		}
	case grepModeContent:
		for _, r := range results {
			for _, ln := range r.lines {
				marker := ":"
				if !ln.match {
					marker = "-"
				}
				fmt.Fprintf(&b, "%s%s%d%s%s\n", r.relPath, marker, ln.num, marker, ln.text)
			}
		}
	}
	return SuccessResult(strings.TrimRight(b.String(), "\n"), "text"), nil
}

type grepLine struct {
	num   int
	text  string
	match bool
}

type grepFileResult struct {
	lines []grepLine
	count int
}

// grepFile scans one file and returns matches plus optional context
// lines. Returns an empty result when the file can't be read or no
// matches are found.
func grepFile(path string, re *regexp.Regexp, contextLines, maxMatches int) grepFileResult {
	f, err := os.Open(path)
	if err != nil {
		return grepFileResult{}
	}
	defer f.Close()

	if isLikelyBinary(f) {
		return grepFileResult{}
	}

	if _, err := f.Seek(0, 0); err != nil {
		return grepFileResult{}
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)

	var all []string
	for scanner.Scan() {
		all = append(all, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return grepFileResult{}
	}

	var matchIdx []int
	for i, line := range all {
		if re.MatchString(line) {
			matchIdx = append(matchIdx, i)
			if len(matchIdx) >= maxMatches {
				break
			}
		}
	}
	if len(matchIdx) == 0 {
		return grepFileResult{}
	}

	picked := make(map[int]bool)
	for _, idx := range matchIdx {
		for off := -contextLines; off <= contextLines; off++ {
			j := idx + off
			if j >= 0 && j < len(all) {
				picked[j] = true
			}
		}
	}

	matchSet := make(map[int]bool, len(matchIdx))
	for _, idx := range matchIdx {
		matchSet[idx] = true
	}

	indices := make([]int, 0, len(picked))
	for k := range picked {
		indices = append(indices, k)
	}
	sort.Ints(indices)

	out := make([]grepLine, 0, len(indices))
	for _, i := range indices {
		out = append(out, grepLine{num: i + 1, text: all[i], match: matchSet[i]})
	}
	return grepFileResult{lines: out, count: len(matchIdx)}
}

// isLikelyBinary peeks at the first 512 bytes of f and returns true if
// any NUL byte is present — a cheap heuristic that keeps the grep tool
// from dumping binary noise into tool output.
func isLikelyBinary(f *os.File) bool {
	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	for _, b := range buf[:n] {
		if b == 0 {
			return true
		}
	}
	return false
}
