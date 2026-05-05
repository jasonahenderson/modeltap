package bff

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jasonahenderson/modeltap/internal/protocol"
	"github.com/jasonahenderson/modeltap/internal/provider"
)

// transformPrompts maps a transform name to its instruction prefix. The
// harness sends a transform name like "summarize" or "extract_facts";
// the BFF wraps the raw content in a per-transform prompt before
// calling the cheap model.
var transformPrompts = map[string]string{
	"summarize":     "Summarize the following content concisely. Focus on the salient facts and remove noise.",
	"extract_facts": "Extract bullet-pointed factual claims from the following content. One claim per bullet.",
	"classify":      "Classify the following content into a short category label and explain in one sentence.",
}

// handleContentTransform implements content.transform per Bundle 11
// design D2 / Bundle 8 design D4.3. The harness sends raw content the
// model never sees in full (e.g., a 50KB pasted log) and asks the
// server to summarize / extract / classify it. The result lands in the
// next turn.submit's content field.
//
// Routing: prefers the routing key "cheap"; falls back to the default
// model. The chosen model's provider receives a one-off non-streaming
// request via TurnDispatcher.DispatchSync.
func handleContentTransform(ctx context.Context, conn *Connection, params json.RawMessage) (any, error) {
	var req protocol.ContentTransform
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &TransportError{Code: CodeInvalidParams, Message: "decode content.transform: " + err.Error()}
	}
	if req.Transform == "" {
		return nil, &TransportError{Code: CodeInvalidParams, Message: "transform is required"}
	}
	if req.RawContent == "" {
		return nil, &TransportError{Code: CodeInvalidParams, Message: "raw_content is required"}
	}

	srv := conn.server

	// Resolve the model: prefer "cheap" routing slot, else "default".
	models, _, _ := srv.routing.Resolve("cheap")
	if len(models) == 0 {
		models, _, _ = srv.routing.Resolve("default")
	}
	if len(models) == 0 {
		return nil, &TransportError{
			Code:    CodeModelUnavailable,
			Message: "no model resolved by routing for content.transform (need 'cheap' or 'default')",
		}
	}
	modelName := models[0]
	entry := srv.models.Get(modelName)
	if entry == nil {
		return nil, NewDiagnosticError(
			CodeModelUnavailable,
			fmt.Sprintf("model %q not in registry", modelName),
			protocol.DiagModelUnavailable,
			"model",
			"cheap-model routing target is missing from the registry",
		)
	}

	// Build a single-turn conversation with the transform instruction
	// as the system prompt and the raw content as the user message.
	instruction := transformPrompts[req.Transform]
	if instruction == "" {
		instruction = fmt.Sprintf("Apply the transform %q to the following content.", req.Transform)
	}
	convo := NewConversation("transform")
	convo.appendMessageForTest("user", req.RawContent)

	maxTokens := req.MaxOutputTokens
	if maxTokens <= 0 {
		maxTokens = 1024
	}

	body, err := srv.dispatch.DispatchSync(ctx, DispatchOpts{
		Conversation: convo,
		SystemPrompt: instruction,
		Model:        modelName,
		EndpointName: entry.Provider,
		MaxTokens:    maxTokens,
		Stream:       false,
		WindowSize:   entry.Info.ContextWindow,
	})
	if err != nil {
		return nil, err
	}

	// Extract text from the provider response. We don't have a generic
	// non-streaming response parser yet; do a best-effort extraction
	// for common provider shapes (Anthropic's content[0].text and
	// OpenAI's choices[0].message.content).
	content := extractTransformText(body)
	cost := 0.0
	if srv.cost != nil {
		// Token counts are unknown from the raw body for the simple
		// extraction; fall back to estimates.
		inputEst := int64(provider.EstimateTokens(req.RawContent + instruction))
		outputEst := int64(provider.EstimateTokens(content))
		cost = srv.cost.ComputeTurnCost(modelName, inputEst, outputEst)
	}

	return &protocol.ContentTransformResponse{
		Content:   content,
		ModelUsed: modelName,
		Cost:      cost,
	}, nil
}

// extractTransformText pulls the text payload out of an Anthropic or
// OpenAI non-streaming response. Keeps the hot path simple: handlers
// can always replace this with a typed parser when needed.
func extractTransformText(body []byte) string {
	// Anthropic shape: {"content":[{"type":"text","text":"..."}], ...}
	var anth struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(body, &anth); err == nil {
		var sb strings.Builder
		for _, c := range anth.Content {
			if c.Type == "text" {
				sb.WriteString(c.Text)
			}
		}
		if sb.Len() > 0 {
			return sb.String()
		}
	}
	// OpenAI shape: {"choices":[{"message":{"content":"..."}}], ...}
	var oai struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &oai); err == nil && len(oai.Choices) > 0 {
		return oai.Choices[0].Message.Content
	}
	return ""
}
