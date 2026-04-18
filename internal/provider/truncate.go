package provider

// Truncate applies age-based truncation and tool-pair reconciliation to
// a canonical message slice. It returns a new slice containing only the
// messages that fit within windowSize tokens (including the system prompt
// budget). The algorithm is described in design doc D4.
func Truncate(msgs []Message, systemPrompt string, windowSize int) ([]Message, error) {
	if len(msgs) == 0 {
		return nil, ErrEmptyMessages
	}

	systemBudget := EstimateTokens(systemPrompt)
	if systemBudget >= windowSize {
		return nil, ErrWindowTooSmall
	}

	remaining := windowSize - systemBudget

	// Stage 2: age-based cutoff — walk from newest to oldest.
	cumulative := 0
	cutoff := 0
	for i := len(msgs) - 1; i >= 0; i-- {
		cost := EstimateMessageTokens(msgs[i])
		if cumulative+cost > remaining {
			cutoff = i + 1
			break
		}
		cumulative += cost
	}

	kept := make([]Message, len(msgs[cutoff:]))
	for i, m := range msgs[cutoff:] {
		// Deep copy to avoid mutating the original.
		kept[i] = copyMessage(m)
	}

	// Stage 3: pair reconciliation.
	// Forward pass: collect tool_call IDs from assistant messages.
	seenCallIDs := make(map[string]bool)
	for _, m := range kept {
		if m.Role == "assistant" {
			for _, call := range m.ToolCalls {
				seenCallIDs[call.ID] = true
			}
		}
	}

	// Collect tool_result IDs to find which calls have results.
	seenResultIDs := make(map[string]bool)
	for _, m := range kept {
		for _, r := range m.ToolResults {
			seenResultIDs[r.ToolCallID] = true
		}
	}

	// Reconcile: drop orphan tool_results (no matching call) and
	// orphan tool_calls (no matching result).
	for i := range kept {
		if len(kept[i].ToolResults) > 0 {
			var validResults []ToolResult
			for _, r := range kept[i].ToolResults {
				if seenCallIDs[r.ToolCallID] {
					validResults = append(validResults, r)
				}
			}
			kept[i].ToolResults = validResults
		}
		if kept[i].Role == "assistant" && len(kept[i].ToolCalls) > 0 {
			var validCalls []ToolCall
			for _, c := range kept[i].ToolCalls {
				if seenResultIDs[c.ID] {
					validCalls = append(validCalls, c)
				}
			}
			kept[i].ToolCalls = validCalls
		}
	}

	// Remove empty messages (no content, no tool calls, no tool results).
	var result []Message
	for _, m := range kept {
		if m.Content == "" && len(m.ToolCalls) == 0 && len(m.ToolResults) == 0 {
			continue
		}
		result = append(result, m)
	}

	if len(result) == 0 {
		return nil, ErrTruncationEmpty
	}

	// Check for at least one user message.
	hasUser := false
	for _, m := range result {
		if m.Role == "user" {
			hasUser = true
			break
		}
	}
	if !hasUser {
		return nil, ErrTruncationEmpty
	}

	return result, nil
}

// copyMessage returns a shallow copy of m with new slices for ToolCalls,
// ToolResults, and Attachments.
func copyMessage(m Message) Message {
	cp := m
	if len(m.ToolCalls) > 0 {
		cp.ToolCalls = make([]ToolCall, len(m.ToolCalls))
		copy(cp.ToolCalls, m.ToolCalls)
	}
	if len(m.ToolResults) > 0 {
		cp.ToolResults = make([]ToolResult, len(m.ToolResults))
		copy(cp.ToolResults, m.ToolResults)
	}
	if len(m.Attachments) > 0 {
		cp.Attachments = make([]Attachment, len(m.Attachments))
		copy(cp.Attachments, m.Attachments)
	}
	return cp
}
