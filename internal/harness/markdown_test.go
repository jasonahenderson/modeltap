package harness

import (
	"strings"
	"testing"
	"time"
)

func TestMarkdownRenderer_Render_Basic(t *testing.T) {
	r, err := NewMarkdownRenderer(80)
	if err != nil {
		t.Fatalf("NewMarkdownRenderer: %v", err)
	}
	out, err := r.Render("# Title\n\nHello, world.")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(out, "Title") || !strings.Contains(out, "Hello") {
		t.Errorf("rendered output missing expected content:\n%s", out)
	}
}

func TestMarkdownRenderer_RenderStreaming_HealsIncompleteFence(t *testing.T) {
	r, err := NewMarkdownRenderer(80)
	if err != nil {
		t.Fatalf("renderer: %v", err)
	}
	partial := "Here is code:\n```go\nfunc main() {"
	out, err := r.RenderStreaming(partial)
	if err != nil {
		t.Fatalf("RenderStreaming: %v", err)
	}
	if !strings.Contains(out, "main") {
		t.Errorf("incomplete fence not healed:\n%s", out)
	}
}

func TestMarkdownRenderer_SetWidth_Recreates(t *testing.T) {
	r, _ := NewMarkdownRenderer(80)
	if err := r.SetWidth(120); err != nil {
		t.Fatalf("SetWidth: %v", err)
	}
	if r.width != 120 {
		t.Errorf("width not updated: %d", r.width)
	}
}

func TestMarkdownRenderer_ShouldRedraw_Debounce(t *testing.T) {
	r, _ := NewMarkdownRenderer(80)
	r.SetDebounce(20 * time.Millisecond)

	if !r.ShouldRedraw() {
		t.Errorf("first ShouldRedraw should return true")
	}
	if r.ShouldRedraw() {
		t.Errorf("immediate second ShouldRedraw should return false (debounced)")
	}
	if !r.Pending() {
		t.Errorf("Pending should be true after a debounced redraw was skipped")
	}
	time.Sleep(30 * time.Millisecond)
	if !r.ShouldRedraw() {
		t.Errorf("after debounce window, ShouldRedraw should return true")
	}
	if r.Pending() {
		t.Errorf("Pending should clear after a successful redraw")
	}
}

func TestHealPartialMarkdown(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "unclosed fence",
			in:   "```go\nfunc main() {",
			want: "```go\nfunc main() {\n```",
		},
		{
			name: "balanced fence untouched",
			in:   "```\nx\n```",
			want: "```\nx\n```",
		},
		{
			name: "unclosed inline code",
			in:   "Use `foo to do bar",
			want: "Use `foo to do bar`",
		},
		{
			name: "unclosed bold",
			in:   "this is **important",
			want: "this is **important**",
		},
		{
			// Italic with _ is intentionally not healed — too many
			// false positives in real text (snake_case, file paths,
			// etc.). Document the no-heal contract explicitly.
			name: "italic underscore left alone",
			in:   "tail _italic",
			want: "tail _italic",
		},
		{
			name: "no healing needed",
			in:   "plain text",
			want: "plain text",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, _ := healPartialMarkdown(c.in)
			if got != c.want {
				t.Errorf("healPartialMarkdown(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestCountInlineBackticks_SkipsFenced(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"", 0},
		{"`a`", 2},
		{"```\ncode\n```", 0},
		{"`a` and ```\nb\n``` then `c`", 4},
	}
	for _, c := range cases {
		if got := countInlineBackticks(c.in); got != c.want {
			t.Errorf("countInlineBackticks(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}
