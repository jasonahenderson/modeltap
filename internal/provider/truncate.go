package provider

// Truncate applies age-based context window truncation with tool-pair
// reconciliation. It returns a subset of msgs that fits within
// windowSize tokens (including the system prompt budget).
//
// Algorithm:
//  1. Budget check: if system prompt alone exceeds windowSize, return ErrWindowTooSmall.
//  2. Age-based cutoff: walk from newest to oldest, keeping messages while
//     cumulative tokens + systemBudget <= windowSize.
//  3. Pair reconciliation: drop orphan tool_results (no matching tool_use)
//     and orphan tool_calls (no matching tool_result). Remove empty messages.
//  4. If no messages remain, return ErrTruncationEmpty.
func Truncate(msgs []Message, systemPrompt string, windowSize int) ([]Message, error) {
	if len(msgs) == 0 {
		return nil, ErrEmptyMessages
	}

	// If no window size specified, return all messages unchanged.
	if windowSize <= 0 {
		return msgs, nil
	}

	systemBudget := EstimateTokens(systemPrompt)
	if systemBudget >= windowSize {
		return nil, ErrWindowTooSmall
	}

	remaining := windowSize - systemBudget

	// Stage 2: age-based cutoff — walk from newest to oldest.
	cumulative := 0
	cutoff := len(msgs) // start with nothing kept
	for i := len(msgs) - 1; i >= 0; i-- {
		msgTokens := EstimateMessageTokens(msgs[i])
		if cumulative+msgTokens > remaining {
			break
		}
		cumulative += msgTokens
		cutoff = i
	}

	if cutoff >= len(msgs) {
		return nil, ErrTruncationEmpty
	}

	// Make a deep-ish copy of the kept slice so we can mutate ToolCalls/ToolResults.
	kept := make([]Message, len(msgs)-cutoff)
	for i, m := range msgs[cutoff:] {
		kept[i] = m
		// Copy slices so mutations don't affect the original.
		if len(m.ToolCalls) > 0 {
			tc := make([]ToolCall, len(m.ToolCalls))
			copy(tc, m.ToolCalls)
			kept[i].ToolCalls = tc
		}
		if len(m.ToolResults) > 0 {
			tr := make([]ToolResult, len(m.ToolResults))
			copy(tr, m.ToolResults)
			kept[i].ToolResults = tr
		}
	}

	// Stage 3: pair reconciliation.
	// Forward pass: collect all tool_use IDs from assistant messages.
	toolUseIDs := make(map[string]bool)
	for _, m := range kept {
		if m.Role == "assistant" {
			for _, call := range m.ToolCalls {
				toolUseIDs[call.ID] = true
			}
		}
	}

	// Collect all tool_result IDs that exist.
	toolResultIDs := make(map[string]bool)
	for _, m := range kept {
		for _, result := range m.ToolResults {
			toolResultIDs[result.ToolCallID] = true
		}
	}

	// Drop orphan tool_results (no matching tool_use in kept).
	for i := range kept {
		if len(kept[i].ToolResults) > 0 {
			filtered := kept[i].ToolResults[:0]
			for _, r := range kept[i].ToolResults {
				if toolUseIDs[r.ToolCallID] {
					filtered = append(filtered, r)
				}
			}
			kept[i].ToolResults = filtered
		}
	}

	// Drop orphan tool_calls (no matching tool_result in kept).
	for i := range kept {
		if len(kept[i].ToolCalls) > 0 {
			filtered := kept[i].ToolCalls[:0]
			for _, c := range kept[i].ToolCalls {
				if toolResultIDs[c.ID] {
					filtered = append(filtered, c)
				}
			}
			kept[i].ToolCalls = filtered
		}
	}

	// Remove messages that are now empty (no content, no tool_calls, no tool_results).
	result := kept[:0]
	for _, m := range kept {
		if m.Content != "" || len(m.ToolCalls) > 0 || len(m.ToolResults) > 0 || len(m.Attachments) > 0 {
			result = append(result, m)
		}
	}

	if len(result) == 0 {
		return nil, ErrTruncationEmpty
	}

	// Check we have at least one user message.
	hasUser := false
	for _, m := range result {
		if m.Role == "user" || m.Role == "tool" {
			hasUser = true
			break
		}
	}
	if !hasUser {
		return nil, ErrTruncationEmpty
	}

	return result, nil
}
