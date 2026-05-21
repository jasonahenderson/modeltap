package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/jasonahenderson/modeltap/internal/protocol"
)

// OllamaProvider implements the Provider interface for the Ollama
// /api/chat endpoint per WU-066. Ollama follows an OpenAI-ish chat
// shape with a couple of differences (NDJSON streaming with one JSON
// object per line, no `data:` SSE prefix; `done` field instead of a
// stop reason; `eval_count` / `prompt_eval_count` for token usage).
type OllamaProvider struct{}

// NewOllamaProvider constructs the Ollama adapter.
func NewOllamaProvider() *OllamaProvider { return &OllamaProvider{} }

// Name returns "ollama".
func (o *OllamaProvider) Name() string { return "ollama" }

// Detect returns true when the request targets a local Ollama daemon.
func (o *OllamaProvider) Detect(r *http.Request) bool {
	if strings.Contains(r.URL.Path, "/api/chat") || strings.Contains(r.URL.Path, "/api/generate") {
		return true
	}
	return false
}

// ollamaChatRequest is the on-the-wire shape we POST to /api/chat.
type ollamaChatRequest struct {
	Model    string         `json:"model"`
	Messages []ollamaMsg    `json:"messages"`
	Stream   bool           `json:"stream"`
	Options  map[string]any `json:"options,omitempty"`
}

type ollamaMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ollamaChatResponse is one streamed object from /api/chat. When stream
// is true Ollama writes one of these per line; when stream is false it
// writes a single object.
type ollamaChatResponse struct {
	Model           string    `json:"model"`
	Message         ollamaMsg `json:"message"`
	Done            bool      `json:"done"`
	DoneReason      string    `json:"done_reason"`
	PromptEvalCount int       `json:"prompt_eval_count"`
	EvalCount       int       `json:"eval_count"`
}

// ParseRequest extracts metadata from an outgoing /api/chat request.
func (o *OllamaProvider) ParseRequest(body []byte, _ http.Header) (*RequestMetadata, error) {
	var req ollamaChatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("ollama: parse request: %w", err)
	}
	return &RequestMetadata{
		Model:    req.Model,
		Messages: len(req.Messages),
		Stream:   req.Stream,
	}, nil
}

// ParseResponse extracts metadata from a non-streaming /api/chat
// response.
func (o *OllamaProvider) ParseResponse(body []byte, _ http.Header, _ int) (*ResponseMetadata, error) {
	var resp ollamaChatResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("ollama: parse response: %w", err)
	}
	return &ResponseMetadata{
		Model:        resp.Model,
		InputTokens:  int64(resp.PromptEvalCount),
		OutputTokens: int64(resp.EvalCount),
		StopReason:   resp.DoneReason,
	}, nil
}

// ReassembleStream walks the captured chunks (one object per chunk),
// joining message.content. Token counts come from the final chunk.
func (o *OllamaProvider) ReassembleStream(chunks []StreamChunk) (*ResponseMetadata, string, error) {
	meta := &ResponseMetadata{}
	var sb strings.Builder
	for _, ch := range chunks {
		// Each chunk.Data is a single JSON object (NDJSON line).
		var obj ollamaChatResponse
		if err := json.Unmarshal(ch.Data, &obj); err != nil {
			continue
		}
		if obj.Model != "" {
			meta.Model = obj.Model
		}
		sb.WriteString(obj.Message.Content)
		if obj.Done {
			meta.StopReason = obj.DoneReason
			meta.InputTokens = int64(obj.PromptEvalCount)
			meta.OutputTokens = int64(obj.EvalCount)
		}
	}
	return meta, sb.String(), nil
}

// ParseStreamEvent decodes one NDJSON object from /api/chat. Ollama
// doesn't use SSE framing; the Runtime's SSEParser will still split on
// blank lines, but Ollama emits one JSON per newline-delimited line —
// the relay loops over these without intermediate framing.
func (o *OllamaProvider) ParseStreamEvent(data []byte) (*StreamEvent, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var obj ollamaChatResponse
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, fmt.Errorf("ollama: parse stream: %w", err)
	}
	if obj.Done {
		ev := &StreamEvent{Type: StreamEventDone}
		if obj.PromptEvalCount > 0 || obj.EvalCount > 0 {
			ev.Usage = &StreamUsage{
				InputTokens:  obj.PromptEvalCount,
				OutputTokens: obj.EvalCount,
			}
		}
		return ev, nil
	}
	if obj.Message.Content == "" {
		return nil, nil
	}
	return &StreamEvent{Type: StreamEventText, Content: obj.Message.Content}, nil
}

// FormatMessages translates a canonical conversation to /api/chat shape.
func (o *OllamaProvider) FormatMessages(opts FormatMessagesOpts) ([]byte, error) {
	if len(opts.Messages) == 0 {
		return nil, ErrEmptyMessages
	}
	msgs := opts.Messages
	if opts.WindowSize > 0 {
		truncated, err := Truncate(msgs, opts.SystemPrompt, opts.WindowSize)
		if err != nil {
			return nil, err
		}
		msgs = truncated
	}

	wire := make([]ollamaMsg, 0, len(msgs)+1)
	if opts.SystemPrompt != "" {
		wire = append(wire, ollamaMsg{Role: "system", Content: opts.SystemPrompt})
	}
	for _, m := range msgs {
		// Map canonical role to Ollama's chat roles. Tool results are
		// flattened into the user message text — Ollama lacks a tool
		// role, but the model still gets the content.
		role := m.Role
		if role == "tool" {
			role = "user"
		}
		content := m.Content
		if len(m.ToolResults) > 0 {
			var sb strings.Builder
			sb.WriteString(content)
			for _, r := range m.ToolResults {
				if sb.Len() > 0 {
					sb.WriteString("\n\n")
				}
				fmt.Fprintf(&sb, "[tool result %s status=%s]\n%s", r.ToolCallID, r.Status, r.Output)
			}
			content = sb.String()
		}
		wire = append(wire, ollamaMsg{Role: role, Content: content})
	}

	payload := ollamaChatRequest{
		Model:    opts.Model,
		Messages: wire,
		Stream:   opts.Stream,
	}
	if opts.MaxTokens > 0 || opts.Temperature != nil {
		payload.Options = map[string]any{}
		if opts.MaxTokens > 0 {
			payload.Options["num_predict"] = opts.MaxTokens
		}
		if opts.Temperature != nil {
			payload.Options["temperature"] = *opts.Temperature
		}
	}
	return json.Marshal(payload)
}

// FormatToolDefinitions returns an empty array — Ollama tool-use is
// model-specific and not consistently exposed across local models.
// Future work: thread through Ollama's `tools` block once a model
// catalog with reliable support emerges.
func (o *OllamaProvider) FormatToolDefinitions(_ []protocol.ToolDefinition) ([]byte, error) {
	return []byte("[]"), nil
}
