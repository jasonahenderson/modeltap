package protocol

// This file declares compaction-related types for WU-041. CompactPlan
// serves double duty as both the response to session.compact and the
// payload of the compact.plan streaming event.

// CompactCategory is an element of CompactPlan.Categories.
type CompactCategory struct {
	Name            string  `json:"name"`
	TokenCount      int     `json:"token_count"`
	ValueScore      float64 `json:"value_score"`
	SuggestedAction string  `json:"suggested_action"`
	SummaryPreview  string  `json:"summary_preview,omitempty"`
}

// CompactFileBreakdown is an element of CompactPlan.FilesBreakdown.
type CompactFileBreakdown struct {
	Path            string `json:"path"`
	TokenCount      int    `json:"token_count"`
	AttachedTurn    int    `json:"attached_turn"`
	Stale           bool   `json:"stale"`
	SuggestedAction string `json:"suggested_action"`
}

// CompactPlan is the response to session.compact AND the payload of the
// compact.plan streaming event. A single declaration serves both paths.
type CompactPlan struct {
	Categories           []CompactCategory      `json:"categories"`
	FilesBreakdown       []CompactFileBreakdown `json:"files_breakdown,omitempty"`
	EstimatedTokensFreed int                    `json:"estimated_tokens_freed"`
	ContextPctBefore     float64                `json:"context_pct_before"`
	ContextPctAfter      float64                `json:"context_pct_after"`
}

// CompactApplyResponse is the response to compact.apply.
type CompactApplyResponse struct {
	Applied         bool    `json:"applied"`
	TokensFreed     int     `json:"tokens_freed"`
	ContextPctAfter float64 `json:"context_pct_after"`
	Summary         string  `json:"summary"`
}
